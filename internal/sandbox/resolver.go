package sandbox

import (
	"context"
	"net"
)

// Resolver is an interface used to implement service name resolution on System
// instances.
//
// net.Resolver is a valid implementation of this interface.
type Resolver interface {
	LookupPort(ctx context.Context, network, service string) (int, error)
	LookupIP(ctx context.Context, network, hostname string) ([]net.IP, error)
}

type defaultResolver struct{}

func (defaultResolver) LookupPort(ctx context.Context, network, service string) (int, error) {
	return 0, &net.DNSError{
		Err:        "service not implemented",
		IsNotFound: true,
	}
}

func (defaultResolver) LookupIP(ctx context.Context, network, hostname string) ([]net.IP, error) {
	return nil, &net.DNSError{
		Err:        "service not implemented",
		Name:       hostname,
		IsNotFound: true,
	}
}
