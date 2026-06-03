package di

import "github.com/scality/virtualip-manager/pkg/usecase"

func (c *Container) GetGenerateConfigUseCase() *usecase.GenerateConfig {
	if c.generateConfigUseCase == nil {
		c.generateConfigUseCase = usecase.NewGenerateConfig(
			c.GetLogger(),
			c.GetConfigGenerator(),
			c.getOutputFilePath(),
		)
	}

	return c.generateConfigUseCase
}
