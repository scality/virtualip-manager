package di

import (
	"github.com/scality/virtualip-manager/pkg/infrastructure/interfacegetter"
	"github.com/scality/virtualip-manager/pkg/service"
)

func (c *Container) getInterfaceGetter() service.InterfaceGetter {
	if c.interfaceGetter == nil {
		c.interfaceGetter = interfacegetter.NewHostNetwork(
			c.GetLogger(),
		)
	}
	return c.interfaceGetter
}
