package main

import (
	"flag"
	"log"

	"webgo/internal/assets/packer"
)

func main() {
	configPath := flag.String("config", "config.example.json", "path to configuration file")
	webDir := flag.String("web", "web", "source web directory")
	buildDir := flag.String("build", "build", "build output directory")
	flag.Parse()

	if err := packer.Run(*configPath, *webDir, *buildDir); err != nil {
		log.Fatalf("pack assets: %v", err)
	}
}
