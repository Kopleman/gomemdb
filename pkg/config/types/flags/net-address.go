package flags

import (
	"errors"
	"net"
)

type NetAddress struct {
	Host string
	Port string
}

func (a *NetAddress) String() string {
	if a.Host == "" && a.Port == "" {
		return ""
	}
	return a.Host + ":" + a.Port
}

func (a *NetAddress) Set(s string) error {
	if s == "" {
		return nil
	}
	host, port, err := net.SplitHostPort(s)

	if err != nil {
		return errors.New("need address in a form host:port")
	}

	if port == "" {
		return errors.New("at least port should be defined")
	}

	if host == "" {
		host = "localhost"
	}

	a.Host = host
	a.Port = port

	return nil
}
