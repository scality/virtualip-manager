package di

import (
	"github.com/scality/virtualip-manager/pkg/infrastructure/configgenerator"
	"github.com/scality/virtualip-manager/pkg/service"
)

func (c *Container) GetConfigGenerator() service.ConfigGenerator {
	if c.configGenerator == nil {
		c.configGenerator = configgenerator.NewKeepalived(
			c.GetLogger(),
			c.getInterfaceGetter(),
			c.config.NodeIP,
			c.config.NodeName,
		)
	}
	return c.configGenerator
}
