package tunnel

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"golang.org/x/crypto/ssh"
)

// hop represents a hop in the tunnel
type hop struct {
	username        string
	host            string
	port            int
	sshClientConfig ssh.ClientConfig
	sshClient       *ssh.Client
}

// userHostPortRegex matches username@host:port
var userHostPortRegex = regexp.MustCompile(`^([^@]+)@([^:]+):(\d+)`)

// errors
var (
	ErrInvalidFormat = errors.New("invalid format")
)

// parseHops just parses the list of hop specs and returns an array of hop elements.
func parseHops(userHostPorts []string) ([]hop, error) {
	var links []hop

	for _, s := range userHostPorts {

		uhp := userHostPortRegex.FindStringSubmatch(s)
		if len(uhp) != 4 {
			return nil, fmt.Errorf("%w: wrong number of elements in [%s]", ErrInvalidFormat, s)
		}

		port, err := strconv.ParseInt(uhp[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidFormat, err)
		}

		link := hop{
			username: uhp[1],
			host:     uhp[2],
			port:     int(port),
		}
		links = append(links, link)

	}

	return links, nil
}

func (l hop) String() string {
	return fmt.Sprintf("%s@%s:%d", l.username, l.host, l.port)
}
