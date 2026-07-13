package config

import (
	"errors"
	"net"
)

func validateServerAddr(v string) error {
	host, port, err := net.SplitHostPort(v)
	if host == "" || port == "" || err != nil {
		return errors.New("server address must be in the format host:port (without specifying a scheme)")
	}
	return nil
}
