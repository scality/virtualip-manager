package interfacegetter

import (
	"log/slog"
	"net"

	"github.com/scality/go-errors"
	"github.com/scality/virtualip-manager/pkg/domain"
	"github.com/scality/virtualip-manager/pkg/service"
)

type HostNetwork struct {
	logger *slog.Logger
}

func NewHostNetwork(logger *slog.Logger) *HostNetwork {
	l := logger.With(
		slog.String("infrastructure", "interfacegetter"),
		slog.String("implementation", "hostnetwork"),
	)
	return &HostNetwork{
		logger: l,
	}
}

var _ service.InterfaceGetter = &HostNetwork{}

// GetInterfaceFromIP returns the name of the host network interface whose
// configured subnet contains the given IP address.
func (h *HostNetwork) GetInterfaceFromIP(ip string) (string, error) {
	target := net.ParseIP(ip)
	if target == nil {
		return "", errors.Wrap(domain.ErrInvalidIPAddress,
			errors.WithDetail("invalid IP address"),
			errors.WithProperty("ip", ip),
		)
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return "",
			errors.Wrap(domain.ErrInterfaceListing,
				errors.WithDetail("failed to list interfaces"),
				errors.CausedBy(err),
			)
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			h.logger.Debug("Failed to list unicast interface addresses for interface",
				"error", err,
				"interface", iface.Name,
			)
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ipNet.Contains(target) {
				return iface.Name, nil
			}
		}
	}

	return "", errors.Wrap(domain.ErrInterfaceNotFound,
		errors.WithDetail("no interface found for IP"),
		errors.WithProperty("ip", ip),
	)
}
