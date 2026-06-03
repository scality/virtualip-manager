package service

import "github.com/scality/virtualip-manager/pkg/domain"

type ConfigGenerator interface {
	// ParseInputData parses the input data and returns a VirtualIPConfig.
	ParseInputData(inputData []byte) (*domain.VirtualIPConfig, error)
	// GenerateConfiguration generates the configuration from the VirtualIPConfig.
	GenerateConfiguration(inputData *domain.VirtualIPConfig) (string, error)
}
