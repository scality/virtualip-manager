package service

type InterfaceGetter interface {
	// GetInterfaceFromIP returns the name of the interface whose
	// configured subnet contains the given IP address.
	GetInterfaceFromIP(ip string) (string, error)
}
