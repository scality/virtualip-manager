package usecase

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/scality/go-errors"
	"github.com/scality/virtualip-manager/pkg/domain"
	"github.com/scality/virtualip-manager/pkg/service"
)

type GenerateConfig struct {
	logger          *slog.Logger
	configGenerator service.ConfigGenerator
	outputFilePath  string
}

func NewGenerateConfig(
	logger *slog.Logger,
	configGenerator service.ConfigGenerator,
	outputFilePath string,
) *GenerateConfig {
	l := logger.With(
		slog.String("usecase", "generate_config"),
	)
	return &GenerateConfig{
		logger:          l,
		configGenerator: configGenerator,
		outputFilePath:  outputFilePath,
	}
}

func (uc *GenerateConfig) Execute(inputFilePath string) error {
	uc.logger.Info("Generating configuration")

	inputData, err := os.ReadFile(inputFilePath)
	if err != nil {
		return errors.Wrap(domain.ErrInputFileReading,
			errors.WithDetail("failed to read input file"),
			errors.WithProperty("inputFilePath", inputFilePath),
			errors.CausedBy(err),
		)
	}

	parsedInputData, err := uc.configGenerator.ParseInputData(inputData)
	if err != nil {
		return errors.Wrap(err,
			errors.WithDetail("failed to parse input data"),
			errors.WithProperty("inputFilePath", inputFilePath),
		)
	}

	outputData, err := uc.configGenerator.GenerateConfiguration(parsedInputData)
	if err != nil {
		return errors.Wrap(err,
			errors.WithDetail("failed to generate configuration"),
			errors.WithProperty("inputFilePath", inputFilePath),
		)
	}

	if uc.outputFilePath == "" {
		fmt.Println(outputData)
		return nil
	}

	if err := os.WriteFile(uc.outputFilePath, []byte(outputData), 0o644); err != nil {
		return errors.Wrap(domain.ErrOutputFileWriting,
			errors.WithDetail("failed to write output file"),
			errors.WithProperty("outputFilePath", uc.outputFilePath),
			errors.CausedBy(err),
		)
	}

	uc.logger.Info("Configuration generated successfully")
	return nil
}
