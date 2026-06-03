package interfacegetter

import (
	"github.com/scality/go-errors"
	"github.com/scality/virtualip-manager/pkg/domain"
	"github.com/scality/virtualip-manager/pkg/service"
)

type MockHostNetwork struct{}

func NewMockHostNetwork() *MockHostNetwork {
	return &MockHostNetwork{}
}

var _ service.InterfaceGetter = &MockHostNetwork{}

// GetInterfaceFromIP returns the name of the host network interface whose
// configured subnet contains the given IP address.
func (h *MockHostNetwork) GetInterfaceFromIP(ip string) (string, error) {
	switch ip {
	case "172.17.0.15":
		return "eth0", nil
	case "172.17.0.16":
		return "eth1", nil
	case "172.17.0.17":
		return "eth2", nil
	default:
		return "", errors.Wrap(domain.ErrInterfaceNotFound,
			errors.WithDetail("no interface found for IP"),
			errors.WithProperty("ip", ip))
	}
}
