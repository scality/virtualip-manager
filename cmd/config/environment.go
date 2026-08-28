package config

import (
	"context"
	"net"

	"github.com/scality/go-errors"
	"github.com/scality/virtualip-manager/pkg/domain"
	"github.com/sethvargo/go-envconfig"
)

// ApplicationVersion is the version of the application.
// It is set at build time using ldflags.
//
//nolint:gochecknoglobals // This is a constant.
var ApplicationVersion = "dev"

const (
	ApplicationName = "virtualip-manager"
)

type (
	Environment struct {
		Logger LoggerConfig `env:",prefix=LOGGER_"`

		NodeIP   string `env:"NODE_IP"`
		NodeName string `env:"NODE_NAME"`

		OutputFilePath string
	}

	// LoggerConfig holds the logging configuration loaded from the environment.
	LoggerConfig struct {
		LogLevel string `env:"LOG_LEVEL, default=info"`
	}
)

func NewEnvironment(ctx context.Context, outputFilePath string) (*Environment, error) {
	cfg := &Environment{
		OutputFilePath: outputFilePath,
	}

	err := cfg.Load(ctx)
	if err != nil {
		return nil, errors.Wrap(err)
	}

	return cfg, nil
}

func (cfg *Environment) Load(ctx context.Context) error {
	err := envconfig.Process(ctx, cfg)
	if err != nil {
		return errors.Wrap(domain.ErrConfigurationLoading,
			errors.WithDetail("failed to process environment variables"),
			errors.CausedBy(err),
		)
	}

	if cfg.NodeIP == "" {
		return errors.Wrap(domain.ErrConfigurationLoading,
			errors.WithDetail("NODE_IP environment variable is required"),
		)
	}

	// NodeIP is interpolated into the generated keepalived config, so reject
	// anything that is not strictly an IP address.
	if net.ParseIP(cfg.NodeIP) == nil {
		return errors.Wrap(domain.ErrInvalidIPAddress,
			errors.WithDetail("NODE_IP is not a valid IP address"),
			errors.WithProperty("nodeIP", cfg.NodeIP),
		)
	}

	if cfg.NodeName == "" {
		return errors.Wrap(domain.ErrConfigurationLoading,
			errors.WithDetail("NODE_NAME environment variable is required"),
		)
	}

	return nil
}
