package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/scality/virtualip-manager/cmd/config"
	"github.com/scality/virtualip-manager/pkg/infrastructure/di"
)

func main() {
	var inputFilePath, outputFilePath string
	flag.StringVar(&inputFilePath, "input", "", "input file path")
	flag.StringVar(&outputFilePath, "output", "", "output file path (defaults to stdout)")
	flag.Parse()
	if inputFilePath == "" {
		fmt.Fprintf(os.Stderr, "input file path is required\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Load configuration from environment variables.
	cfg, err := config.NewEnvironment(context.Background(), outputFilePath)
	if err != nil {
		// %s renders go-errors' human-readable form; the default %v
		// (what Fprintln would use) emits JSON, which is for logs.
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	// Initialize the dependency container.
	container := di.NewContainer(cfg)
	logger := container.GetLogger()

	err = container.GetGenerateConfigUseCase().Execute(inputFilePath)
	if err != nil {
		logger.Error("Failed to generate configuration", "error", err)
		os.Exit(1)
	}
}
