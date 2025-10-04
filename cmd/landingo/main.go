package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elchemista/LandingGo/internal/assets/packer"
)

func main() {
	if len(os.Args) < 2 {
		printRootUsage()
		os.Exit(2)
	}

	sub := os.Args[1]
	args := os.Args[2:]

	var err error
	switch sub {
	case "build":
		err = runBuild(args)
	case "pack":
		err = runPack(args)
	case "help", "-h", "--help":
		printRootUsage()
		return
	default:
		err = fmt.Errorf("unknown command %q", sub)
	}

	if err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func runPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	config := fs.String("config", "config.prod.json", "path to configuration file")
	web := fs.String("web", "web", "path to folder containing pages/static assets")
	buildDir := fs.String("build", "build", "output directory for generated embed files")

	if err := fs.Parse(args); err != nil {
		return usageErr("pack", err)
	}

	logger := log.New(os.Stdout, "", 0)
	logger.Printf("Packing assets from %s with %s", *web, *config)
	start := time.Now()

	if err := packer.Run(*config, *web, *buildDir); err != nil {
		return err
	}

	logger.Printf("Assets packed into %s (took %s)", *buildDir, time.Since(start).Round(time.Millisecond))
	return nil
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	config := fs.String("config", "config.prod.json", "path to configuration file")
	web := fs.String("web", "web", "path to folder containing pages/static assets")
	buildDir := fs.String("build", "build", "output directory for generated embed files")
	output := fs.String("output", filepath.Join("bin", "landing"), "where to write the compiled binary")
	goBinary := fs.String("go", "go", "path to the go toolchain")
	ldflags := fs.String("ldflags", "-s -w", "ldflags passed to go build")
	tags := fs.String("tags", "", "optional build tags (comma separated)")
	trimpath := fs.Bool("trimpath", true, "add -trimpath when compiling")
	skipPack := fs.Bool("skip-pack", false, "skip repacking assets before building")

	if err := fs.Parse(args); err != nil {
		return usageErr("build", err)
	}

	logger := log.New(os.Stdout, "", 0)

	if !*skipPack {
		logger.Printf("Packing assets from %s with %s", *web, *config)
		start := time.Now()
		if err := packer.Run(*config, *web, *buildDir); err != nil {
			return err
		}
		logger.Printf("Assets packed into %s (took %s)", *buildDir, time.Since(start).Round(time.Millisecond))
	}

	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	argsBuild := []string{"build"}
	if *trimpath {
		argsBuild = append(argsBuild, "-trimpath")
	}

	if strings.TrimSpace(*ldflags) != "" {
		argsBuild = append(argsBuild, "-ldflags", *ldflags)
	}

	if strings.TrimSpace(*tags) != "" {
		argsBuild = append(argsBuild, "-tags", *tags)
	}

	argsBuild = append(argsBuild, "-o", *output, "./cmd/landing")

	cmd := exec.Command(*goBinary, argsBuild...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	logger.Printf("Compiling binary to %s", *output)
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}
	logger.Printf("Binary written to %s (took %s)", *output, time.Since(start).Round(time.Millisecond))
	return nil
}

func usageErr(cmd string, err error) error {
	if errors.Is(err, flag.ErrHelp) {
		printCommandUsage(cmd)
		return nil
	}
	printCommandUsage(cmd)
	return err
}

func printRootUsage() {
	fmt.Println(`LandinGo CLI

Usage:
  landingo <command> [options]

Commands:
  build   Pack assets and compile the landing server into a single binary
  pack    Pack assets only (generates embedded files)

Use "landingo <command> -h" for command-specific help.`)
}

func printCommandUsage(cmd string) {
	switch cmd {
	case "build":
		fmt.Println(`Usage: landingo build [options]

Options:
  --config     path to configuration file (default "config.prod.json")
  --web        path to folder containing pages/static assets (default "web")
  --build      output directory for generated embed files (default "build")
  --output     where to write the compiled binary (default "bin/landing")
  --go         path to the go toolchain (default "go")
  --ldflags    ldflags passed to go build (default "-s -w")
  --tags       optional build tags (comma separated)
  --trimpath   add -trimpath when compiling (default true)
  --skip-pack  skip packing assets before building`)
	case "pack":
		fmt.Println(`Usage: landingo pack [options]

Options:
  --config   path to configuration file (default "config.prod.json")
  --web      path to folder containing pages/static assets (default "web")
  --build    output directory for generated embed files (default "build")`)
	default:
		printRootUsage()
	}
}
