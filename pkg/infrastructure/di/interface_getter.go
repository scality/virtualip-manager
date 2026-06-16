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

func (c *Container) GetMockInterfaceGetter() service.InterfaceGetter {
	if c.interfaceGetter == nil {
		c.interfaceGetter = interfacegetter.NewMockHostNetwork()
	}
	return c.interfaceGetter
}
