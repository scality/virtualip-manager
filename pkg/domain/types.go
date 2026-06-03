package domain

import (
	"slices"

	"github.com/scality/go-errors"
)

var (
	EXPECTED_KIND         string   = "VirtualIPConfiguration"
	SUPPORTED_API_VERSION []string = []string{"loadbalancer.scality.com/v1alpha1"}
)

type Address struct {
	Ip   string `yaml:"ip"`
	Node string `yaml:"node"`
	VrId int    `yaml:"vrId"`
}

type VirtualIPConfigMetadata struct {
	ApiVersion *string `yaml:"apiVersion,omitempty"`
	Kind       *string `yaml:"kind,omitempty"`
}

type VirtualIPConfig struct {
	VirtualIPConfigMetadata `yaml:",inline"`

	Addresses   []Address `yaml:"addresses,omitempty"`
	Healthcheck *string   `yaml:"healthcheck,omitempty"`
}

func (v *VirtualIPConfig) Validate() error {
	// Check if the kind is present and if it is the expected kind
	if v.Kind == nil {
		return errors.Wrap(ErrMissingInputParameter,
			errors.WithDetail("missing Kind"),
		)
	}

	if *v.Kind != EXPECTED_KIND {
		return errors.Wrap(ErrInvalidInputParameter,
			errors.WithDetail("invalid Kind"),
			errors.WithProperty("expectedKind", EXPECTED_KIND),
			errors.WithProperty("actualKind", *v.Kind),
		)
	}

	// Check if the apiVersion is present and if it is supported
	if v.ApiVersion == nil {
		return errors.Wrap(ErrMissingInputParameter,
			errors.WithDetail("missing ApiVersion"),
		)
	}

	if !slices.Contains(SUPPORTED_API_VERSION, *v.ApiVersion) {
		return errors.Wrap(ErrInvalidInputParameter,
			errors.WithDetail("invalid ApiVersion"),
			errors.WithProperty("expectedApiVersion", SUPPORTED_API_VERSION),
			errors.WithProperty("actualApiVersion", *v.ApiVersion),
		)
	}

	if len(v.Addresses) == 0 {
		return errors.Wrap(ErrMissingInputParameter,
			errors.WithDetail("empty or missing Addresses"),
		)
	}

	for _, address := range v.Addresses {
		if address.Ip == "" {
			return errors.Wrap(ErrMissingInputParameter,
				errors.WithDetail("missing Ip"),
			)
		}
		if address.Node == "" {
			return errors.Wrap(ErrMissingInputParameter,
				errors.WithDetail("missing Node"),
			)
		}
		if address.VrId == 0 {
			return errors.Wrap(ErrMissingInputParameter,
				errors.WithDetail("missing VrId"),
			)
		}
	}

	return nil
}

func (v *VirtualIPConfig) CleanHealthcheck() {
	if v.Healthcheck != nil && *v.Healthcheck == "" {
		v.Healthcheck = nil
	}
}
