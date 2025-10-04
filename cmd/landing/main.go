package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/elchemista/LandingGo/build"
	"github.com/elchemista/LandingGo/internal/assets"
	"github.com/elchemista/LandingGo/internal/config"
	"github.com/elchemista/LandingGo/internal/log"
	"github.com/elchemista/LandingGo/internal/server"
)

const (
	defaultAddr   = ":8080"
	defaultConfig = "config.prod.json"
	webRoot       = "web"
)

func main() {
	cfg := parseConfig()

	logger := log.New(cfg.logLevel)

	src, err := loadSource(cfg.dev, cfg.folder)
	if err != nil {
		logger.Error("load assets", "error", err)
		os.Exit(1)
	}

	conf, configSource, err := loadConfig(cfg.configPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	if configSource != "" {
		logger.Info("configuration loaded", "source", configSource)
	}

	if err := conf.Validate(func(name string) bool { return src.PageExists(name) }); err != nil {
		logger.Error("validate config", "error", err)
		os.Exit(1)
	}

	srv, err := server.New(conf, src, logger, cfg.dev)
	if err != nil {
		logger.Error("initialise server", "error", err)
		os.Exit(1)
	}

	httpSrv := &http.Server{
		Addr:              cfg.addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	done := make(chan struct{})

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown", "error", err)
		}

		close(done)
	}()

	logger.Info("server starting", "addr", cfg.addr, "dev", cfg.dev)

	err = httpSrv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	<-done
	logger.Info("server stopped")
}

type runtimeConfig struct {
	configPath string
	addr       string
	logLevel   string
	folder     string
	dev        bool
}

type stringFlag struct {
	value string
	set   bool
}

func (s *stringFlag) String() string { return s.value }

func (s *stringFlag) Set(v string) error {
	s.value = strings.TrimSpace(v)
	s.set = true
	return nil
}

func parseConfig() runtimeConfig {
	configDefault := envOrDefault("CONFIG", defaultConfig)
	addrDefault := envOrDefault("ADDR", "")
	if addrDefault == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			if strings.HasPrefix(port, ":") {
				addrDefault = port
			} else {
				addrDefault = ":" + port
			}
		}
	}
	if addrDefault == "" {
		addrDefault = defaultAddr
	}

	logLevelDefault := envOrDefault("LOG_LEVEL", "info")
	devDefault := envBool("DEV", false)
	folderDefault := envOrDefault("FOLDER", "")

	configFlag := &stringFlag{value: configDefault}
	addrFlag := &stringFlag{value: addrDefault}
	folderFlag := &stringFlag{value: folderDefault}

	flag.Var(configFlag, "config", "path to configuration file")
	flag.Var(addrFlag, "addr", "address to listen on (host:port)")
	flag.Var(folderFlag, "folder", "path to the asset folder (overrides embedded assets)")
	logLevel := flag.String("log-level", logLevelDefault, "log level (debug, info, warn, error)")
	dev := flag.Bool("dev", devDefault, "run in development mode (serve assets from disk)")

	flag.Parse()

	return runtimeConfig{
		configPath: configFlag.value,
		addr:       addrFlag.value,
		logLevel:   *logLevel,
		folder:     folderFlag.value,
		dev:        *dev,
	}
}

func envOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	switch strings.ToLower(val) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func loadConfig(path string) (*config.Config, string, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath != "" {
		conf, err := config.Load(cleanPath)
		if err == nil {
			applyRuntimeOverrides(conf)
			return conf, cleanPath, nil
		}

		if !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}
	}

	embedded := build.EmbeddedConfig()
	if len(embedded) == 0 {
		if cleanPath != "" {
			return nil, "", fmt.Errorf("config %s not found and no embedded configuration present", cleanPath)
		}
		return nil, "", errors.New("embedded configuration is missing")
	}

	conf, err := config.Parse(embedded)
	if err != nil {
		return nil, "", fmt.Errorf("parse embedded config: %w", err)
	}

	conf.WithSource("embedded")
	conf.WithLoadedTime(time.Now().UTC())
	applyRuntimeOverrides(conf)

	if cleanPath != "" {
		return conf, fmt.Sprintf("embedded (fallback from %s)", cleanPath), nil
	}

	return conf, "embedded", nil
}

func applyRuntimeOverrides(cfg *config.Config) {
	if cfg == nil {
		return
	}

	if apiKey := strings.TrimSpace(os.Getenv("MAILGUN_API_KEY")); apiKey != "" {
		cfg.Contact.Mailgun.APIKey = apiKey
	}
}

func loadSource(dev bool, folder string) (*assets.Source, error) {
	root := strings.TrimSpace(folder)
	if root == "" && dev {
		root = webRoot
	}

	if root != "" {
		return assets.NewDisk(root)
	}

	sub, err := fs.Sub(build.FS, "public")
	if err != nil {
		return nil, err
	}

	return assets.NewEmbedded(sub)
}
