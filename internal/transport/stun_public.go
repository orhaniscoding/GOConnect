package transport

import "time"

// QueryPublicEndpoint wraps the package-internal STUN query to also return an RTT estimate.
// It sends a single STUN binding request to the given server (host:port) and parses the
// XOR-MAPPED-ADDRESS from the response. If successful, it returns endpoint in IP:port form
// and a coarse round-trip time.
func QueryPublicEndpoint(server string) (endpoint string, rtt time.Duration, err error) {
	start := time.Now()
	ep, err := queryPublicEndpoint(server)
	if err != nil {
		return "", 0, err
	}
	return ep, time.Since(start), nil
}

// ProbePublicEndpoint tries the given servers in order and returns the first successful result.
func ProbePublicEndpoint(servers []string) (server string, endpoint string, rtt time.Duration, err error) {
	var firstErr error
	for _, s := range servers {
		if s == "" {
			continue
		}
		ep, d, err := QueryPublicEndpoint(s)
		if err == nil {
			return s, ep, d, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr == nil {
		firstErr = ErrNoSTUNServers
	}
	return "", "", 0, firstErr
}

// ErrNoSTUNServers indicates no servers were provided for STUN probing.
var ErrNoSTUNServers = errNoSTUNServers{}

type errNoSTUNServers struct{}

func (errNoSTUNServers) Error() string { return "no stun servers configured" }
