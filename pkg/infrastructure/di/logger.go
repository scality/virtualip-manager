package di

import (
	"log/slog"
	"os"

	"github.com/scality/virtualip-manager/cmd/config"
)

func (c *Container) GetLogger() *slog.Logger {
	if c.logger == nil {
		var level slog.Level
		if err := level.UnmarshalText([]byte(c.config.Logger.LogLevel)); err != nil {
			level = slog.LevelInfo
		}

		handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})

		c.logger = slog.New(handler).With(
			slog.String("application_name", config.ApplicationName),
			slog.String("application_version", config.ApplicationVersion),
		)
	}

	return c.logger
}
