package di

import (
	"log/slog"

	"github.com/scality/virtualip-manager/cmd/config"
	"github.com/scality/virtualip-manager/pkg/service"
	"github.com/scality/virtualip-manager/pkg/usecase"
)

type Container struct {
	config *config.Environment

	logger *slog.Logger

	configGenerator       service.ConfigGenerator
	interfaceGetter       service.InterfaceGetter
	generateConfigUseCase *usecase.GenerateConfig
}

func NewContainer(
	cfg *config.Environment,
) *Container {
	return &Container{
		config: cfg,
	}
}
