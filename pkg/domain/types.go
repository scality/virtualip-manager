package domain

import (
	"net/url"
	"slices"
	"strings"

	"github.com/scality/go-errors"
)

var (
	EXPECTED_KIND         string   = "VirtualIPConfiguration"
	SUPPORTED_API_VERSION []string = []string{"loadbalancer.scality.com/v1alpha1"}
	NODE_IP_TOKEN         string   = "__NODE_IP__"
)

// healthcheckForbiddenChars are characters that would either break out of the
// quoted `script "…"` string in the generated keepalived config or be
// interpreted by a shell if keepalived ever falls back to one.
const healthcheckForbiddenChars = "\"'`$;&|<>\\ \t\n\r"

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

	Addresses           []Address `yaml:"addresses,omitempty"`
	Healthcheck         *string   `yaml:"healthcheck,omitempty"`
	HealthCheckNodePort *string   `yaml:"healthcheckNodePort,omitempty"`
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

	if err := validateHealthcheckURL("healthcheck", v.Healthcheck); err != nil {
		return err
	}

	if err := validateHealthcheckURL("healthcheckNodePort", v.HealthCheckNodePort); err != nil {
		return err
	}

	return nil
}

// validateHealthcheckURL rejects a healthcheck that is not an http(s) URL, or
// that carries characters unsafe to interpolate into the keepalived config. The
// NODE_IP_TOKEN is substituted with a placeholder host so the value is
// parseable before the template resolves it.
func validateHealthcheckURL(field string, value *string) error {
	if value == nil {
		return nil
	}

	if strings.ContainsAny(*value, healthcheckForbiddenChars) {
		return errors.Wrap(ErrInvalidInputParameter,
			errors.WithDetail("healthcheck contains forbidden characters"),
			errors.WithProperty("field", field),
			errors.WithProperty("value", *value),
		)
	}

	parsed, err := url.Parse(strings.ReplaceAll(*value, NODE_IP_TOKEN, "0.0.0.0"))
	if err != nil {
		return errors.Wrap(ErrInvalidInputParameter,
			errors.WithDetail("healthcheck is not a valid URL"),
			errors.WithProperty("field", field),
			errors.WithProperty("value", *value),
			errors.CausedBy(err),
		)
	}

	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.Wrap(ErrInvalidInputParameter,
			errors.WithDetail("healthcheck must be an http or https URL with a host"),
			errors.WithProperty("field", field),
			errors.WithProperty("value", *value),
		)
	}

	return nil
}

// CleanHealthchecks normalizes the optional healthcheck fields: a key present
// in the input but left empty is treated as absent, so the template only has
// to test for nil.
func (v *VirtualIPConfig) CleanHealthchecks() {
	if v.Healthcheck != nil && *v.Healthcheck == "" {
		v.Healthcheck = nil
	}

	if v.HealthCheckNodePort != nil && *v.HealthCheckNodePort == "" {
		v.HealthCheckNodePort = nil
	}
}
