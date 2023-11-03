package tunnel

import (
	"errors"
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Tunnel instance.
type Tunnel struct {
	last   *ssh.Client
	config Config
	hops   []hop
}

// Config for Tunnel.
type Config struct {
	// Hops is a list of user@host:port elements, the last of which defines
	// the target host. We need at least one entry, but we support an arbitrary
	// number of hops.
	Hops []string
}

// sshDialerFunc is just a convenient type to make the func signature  for
// sshDialerFromClient look a bit more tidy
type sshDialerFunc func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error)

var (
	// ErrConnectAgent indicates that we failed to connect to the ssh-agent
	ErrConnectAgent = errors.New("failed to connect to ssh-agent")
	// ErrNoHopsSpecified indicates that the caller did not supply any hops. We need at least one hop.
	ErrNoHopsSpecified = errors.New("no hops specified")
	// ErrParsingHops indicates that at least one hop had an improper format
	ErrParsingHops = errors.New("error parsing hops")
	// ErrOpeningAuthSock indicates that we failed to open the ssh-agent socket
	ErrOpeningAuthSock = errors.New("error opening ssh-agent socket")
	// ErrCreatingConnection indicates we were unable to create a connection when building the tunnel
	ErrCreatingConnection = errors.New("error creating connection")
	// ErrClosingHop indicates that we got an error while trying to close a connection when tearing
	// down the tunnel.
	ErrClosingHop = errors.New("error closing hop")
)

// Create new tunnel instance.
func Create(c Config) (*Tunnel, error) {
	if len(c.Hops) == 0 {
		return nil, ErrNoHopsSpecified
	}

	hops, err := parseHops(c.Hops)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParsingHops, err)
	}

	// open connection to ssh agent
	authSockPath := os.Getenv("SSH_AUTH_SOCK")
	conn, err := net.Dial("unix", authSockPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOpeningAuthSock, err)
	}
	agentClient := agent.NewClient(conn)

	tunnel := &Tunnel{
		config: c,
	}

	sshDialer := ssh.Dial
	for _, hop := range hops {
		hop.sshClientConfig = ssh.ClientConfig{
			User: hop.username,
			Auth: []ssh.AuthMethod{
				// TODO(borud): make it possible to override
				ssh.PublicKeysCallback(agentClient.Signers),
			},
			// TODO(borud): make this configurable
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}

		hop.sshClient, err = sshDialer("tcp", fmt.Sprintf("%s:%d", hop.host, hop.port), &hop.sshClientConfig)
		if err != nil {
			tunnel.Shutdown()
			return nil, fmt.Errorf("%w to [%s@%s:%d]: %v", ErrCreatingConnection, hop.username, hop.host, hop.port, err)
		}

		tunnel.hops = append(tunnel.hops, hop)
		tunnel.last = hop.sshClient
		sshDialer = sshDialerFromClient(hop.sshClient)
	}

	return tunnel, nil
}

// Dial from end of tunnel.
func (t *Tunnel) Dial(n string, addr string) (net.Conn, error) {
	return t.last.Dial(n, addr)
}

// Listen to port at end of tunnel.
func (t *Tunnel) Listen(n string, addr string) (net.Listener, error) {
	return t.last.Listen(n, addr)
}

// Shutdown tunnel. This will not shut down any connections you have tunneled through
// so you have to take care of this yourself.
func (t *Tunnel) Shutdown() error {
	var errs error

	// start with the innermost ssh connection and work our way outward.
	for i := len(t.hops) - 1; i >= 0; i-- {
		if t.hops[i].sshClient == nil {
			continue
		}

		err := t.hops[i].sshClient.Close()
		if err != nil {
			errs = errors.Join(errs,
				fmt.Errorf("%w: hop %d [%s@%s:%d]", ErrClosingHop, i, t.hops[i].username, t.hops[i].host, t.hops[i].port))
		}
	}
	return errs
}

// sshDialerFromClient creates a new SSH dialer given a client.
func sshDialerFromClient(client *ssh.Client) sshDialerFunc {
	return func(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
		conn, err := client.Dial(network, addr)
		if err != nil {
			return nil, err
		}

		ncc, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
		if err != nil {
			return nil, err
		}

		return ssh.NewClient(ncc, chans, reqs), nil
	}
}
