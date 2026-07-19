package main

import (
	"fmt"
	"os"

	"github.com/williamokano/claude-status-line-go/internal/config"
	"github.com/williamokano/claude-status-line-go/internal/service"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	svc := service.New(cfg)
	if err := svc.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
