package envutil

import (
	"errors"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ReplaceHost applies envName+"_HOST" to a URL without changing its credentials,
// port, path, or query. An optional override port is validated then ignored.
// Empty overrides leave value unchanged. Errors never include configuration values.
func ReplaceHost(envName, value string) (string, error) {
	override := os.Getenv(envName + "_HOST")
	if override == "" {
		return value, nil
	}
	host, err := overrideHostname(override)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(value)
	if err != nil || u == nil || u.Scheme == "" || u.Hostname() == "" || u.Opaque != "" {
		return "", errors.New("envutil: invalid URL for host override")
	}
	if strings.Contains(u.Hostname(), ":") && !strings.HasPrefix(u.Host, "[") {
		return "", errors.New("envutil: invalid URL host for override")
	}
	if _, err := overrideHostname(u.Host); err != nil {
		return "", errors.New("envutil: invalid URL host for override")
	}
	if port := u.Port(); port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	return u.String(), nil
}

func overrideHostname(value string) (string, error) {
	invalid := errors.New("envutil: invalid host override")
	// Parse bare IPv6 before looking for a port: its last component is not a port.
	if addr, ok := normalizedIP(value); ok {
		return addr, nil
	}
	host := value
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		host = value[1 : len(value)-1]
		addr, ok := normalizedIP(host)
		if !ok || !strings.Contains(addr, ":") {
			return "", invalid
		}
		return addr, nil
	}
	if strings.Contains(value, ":") {
		var port string
		var err error
		host, port, err = net.SplitHostPort(value)
		if err != nil || port == "" {
			return "", invalid
		}
		for _, c := range port {
			if c < '0' || c > '9' {
				return "", invalid
			}
		}
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return "", invalid
		}
		if addr, ok := normalizedIP(host); ok {
			return addr, nil
		}
	}
	if len(host) == 0 || len(host) > 253 {
		return "", invalid
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", invalid
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
				return "", invalid
			}
		}
	}
	return host, nil
}

func normalizedIP(value string) (string, bool) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", false
	}
	for _, c := range addr.Zone() {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return "", false
		}
	}
	return addr.String(), true
}
