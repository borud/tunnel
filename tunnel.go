// Package tunnel is a very simple library that allows you to create multi-hop SSH tunnels.
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Tunnel implements an SSH tunneling helper.
type Tunnel struct {
	cfg     Config
	clients []*ssh.Client

	mu        sync.Mutex
	connTrack map[net.Conn]struct{}
	closed    atomic.Bool

	_listeners map[net.Listener]struct{}
}

// Hop describes one SSH jump (user@host:port).
type Hop struct {
	User            string
	HostPort        string
	HostKeyCallback ssh.HostKeyCallback
	KnownHostsPath  string
	Timeout         time.Duration
}

// Create a new tunnel
func Create(opts ...Option) (*Tunnel, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	if len(cfg.Hops) == 0 {
		return nil, ErrNoHops
	}
	// require at least one auth method: signer or agent (explicitly enabled)
	if len(cfg.Signers) == 0 && !cfg.UseAgent {
		return nil, fmt.Errorf("%w: provide WithSigner/WithKeyFile or WithAgent()", ErrNoAuth)
	}

	return &Tunnel{
		cfg:        cfg,
		connTrack:  make(map[net.Conn]struct{}),
		_listeners: make(map[net.Listener]struct{}),
	}, nil
}

// Dial dials a remote address through the tunnel using a background context.
func (t *Tunnel) Dial(network, addr string) (net.Conn, error) {
	return t.DialContext(context.Background(), network, addr)
}

// Listen starts a remote listener on the last hop using a background context.
func (t *Tunnel) Listen(network, laddr string) (net.Listener, error) {
	return t.ListenContext(context.Background(), network, laddr)
}

// DialContext dials a remote address through the tunnel.
func (t *Tunnel) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if err := t.ensureChain(ctx); err != nil {
		return nil, err
	}
	last := t.clients[len(t.clients)-1]
	conn, err := last.Dial(network, addr)
	if err != nil {
		return nil, fmt.Errorf("dial remote %s %s: %w", network, addr, err)
	}
	return t.track(conn), nil
}

// ListenContext asks the last hop in the tunnel to start listening on laddr.
// Example: ("tcp", "0.0.0.0:8080") will bind a TCP listener on the remote
// side. The returned net.Listener accepts connections forwarded back through
// the tunnel.
//
// For remote listening to work, the SSH server on the last hop must allow it:
// GatewayPorts yes and AllowTcpForwarding yes in sshd_config.
func (t *Tunnel) ListenContext(ctx context.Context, network, laddr string) (net.Listener, error) {
	if err := t.ensureChain(ctx); err != nil {
		return nil, err
	}
	last := t.clients[len(t.clients)-1]

	ln, err := last.Listen(network, laddr)
	if err != nil {
		return nil, fmt.Errorf("listen remote %s %s: %w", network, laddr, err)
	}
	t.trackListener(ln)

	// Close the listener when ctx is canceled.
	if ctx != nil {
		go func() {
			<-ctx.Done()
			_ = ln.Close()
		}()
	}

	return ln, nil
}

// LocalForward listens locally and forwards to a remote address via the tunnel.
func (t *Tunnel) LocalForward(ctx context.Context, laddr, raddr string) (net.Listener, error) {
	if err := t.ensureChain(ctx); err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", laddr, err)
	}
	t.trackListener(ln)
	go t.serveForward(ctx, ln, raddr)
	return ln, nil
}

// Close closes all clients, tracked conns, and listeners.
func (t *Tunnel) Close() error {
	if t.closed.Swap(true) {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	var errs []error
	for ln := range t._listeners {
		if err := ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, err)
		}
		delete(t._listeners, ln)
	}
	if t.cfg.TrackConns {
		for c := range t.connTrack {
			_ = c.Close()
			delete(t.connTrack, c)
		}
	}
	for i := len(t.clients) - 1; i >= 0; i-- {
		if err := t.clients[i].Close(); err != nil {
			errs = append(errs, fmt.Errorf("close hop %d: %w", i, err))
		}
	}
	t.clients = nil
	return errors.Join(errs...)
}

func (t *Tunnel) ensureChain(ctx context.Context) error {
	if t.closed.Load() {
		return ErrClosed
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.clients) > 0 {
		return nil
	}

	authMethods, err := t.authMethods()
	if err != nil {
		return err
	}

	var prevClient *ssh.Client
	dialer := &net.Dialer{Timeout: t.cfg.PerHopTimeout}

	for i, hop := range t.cfg.Hops {
		timeout := hop.Timeout
		if timeout == 0 {
			timeout = t.cfg.PerHopTimeout
		}
		hkcb, err := t.resolveHostKeyCallback(hop)
		if err != nil {
			return fmt.Errorf("hop %d known_hosts: %w", i, err)
		}
		cc := &ssh.ClientConfig{
			User:            hop.User,
			Auth:            authMethods,
			HostKeyCallback: hkcb,
			Timeout:         timeout,
		}

		var underlay net.Conn
		if i == 0 {
			underlay, err = dialer.DialContext(ctx, "tcp", hop.HostPort)
			if err != nil {
				return fmt.Errorf("dial hop %d (%s): %w", i, hop.HostPort, err)
			}
		} else {
			underlay, err = prevClient.Dial("tcp", hop.HostPort)
			if err != nil {
				return fmt.Errorf("via hop %d dial next %s: %w", i-1, hop.HostPort, err)
			}
		}

		conn, chans, reqs, err := ssh.NewClientConn(underlay, hop.HostPort, cc)
		if err != nil {
			_ = underlay.Close()
			return fmt.Errorf("ssh handshake hop %d (%s): %w", i, hop.HostPort, err)
		}
		client := ssh.NewClient(conn, chans, reqs)
		prevClient = client
		t.clients = append(t.clients, client)

		if t.cfg.KeepAlive > 0 {
			go keepAlive(client, t.cfg.KeepAlive)
		}
	}

	return nil
}

func (t *Tunnel) authMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if len(t.cfg.Signers) > 0 {
		methods = append(methods, ssh.PublicKeys(t.cfg.Signers...))
	}
	if t.cfg.UseAgent {
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			if conn, err := net.Dial("unix", sock); err == nil {
				ag := agent.NewClient(conn)
				methods = append(methods, ssh.PublicKeysCallback(ag.Signers))
			}
		}
	}
	if len(methods) == 0 {
		return nil, ErrNoAuth
	}
	return methods, nil
}

func (t *Tunnel) resolveHostKeyCallback(h Hop) (ssh.HostKeyCallback, error) {
	if h.HostKeyCallback != nil {
		return h.HostKeyCallback, nil
	}
	if h.KnownHostsPath != "" {
		return knownhosts.New(h.KnownHostsPath)
	}
	if t.cfg.HostKeyCB != nil {
		return t.cfg.HostKeyCB, nil
	}
	path := t.cfg.KnownHostsPath
	if path == "" {
		var err error
		path, err = defaultKnownHostsPath()
		if err != nil {
			return nil, err
		}
	}
	return knownhosts.New(path)
}

func (t *Tunnel) serveForward(ctx context.Context, ln net.Listener, raddr string) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		c = t.track(c)
		go func(lc net.Conn) {
			defer t.untrack(lc)
			remote, err := t.DialContext(ctx, "tcp", raddr)
			if err != nil {
				_ = lc.Close()
				return
			}
			defer t.untrack(remote)
			pipe(lc, remote)
		}(c)
	}
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		io.Copy(a, b)
		_ = a.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()
	go func() {
		io.Copy(b, a)
		_ = b.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()
	wg.Wait()
	_ = a.Close()
	_ = b.Close()
}

func keepAlive(c *ssh.Client, d time.Duration) {
	t := time.NewTicker(d)
	defer t.Stop()
	for range t.C {
		_, _, err := c.SendRequest("keepalive@openssh.com", true, nil)
		if err != nil {
			return
		}
	}
}

func (t *Tunnel) track(conn net.Conn) net.Conn {
	if !t.cfg.TrackConns {
		return conn
	}
	t.mu.Lock()
	t.connTrack[conn] = struct{}{}
	t.mu.Unlock()
	return &trackedConn{Conn: conn, onClose: func() {
		t.mu.Lock()
		delete(t.connTrack, conn)
		t.mu.Unlock()
	}}
}

func (t *Tunnel) untrack(conn net.Conn) {
	if !t.cfg.TrackConns {
		_ = conn.Close()
		return
	}
	_ = conn.Close()
}

func (t *Tunnel) trackListener(ln net.Listener) {
	t.mu.Lock()
	t._listeners[ln] = struct{}{}
	t.mu.Unlock()
}

func defaultKnownHostsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		if h := os.Getenv("HOME"); h != "" {
			return h + "/.ssh/known_hosts", nil
		}
		return "", fmt.Errorf("resolve home dir for known_hosts: %w", err)
	}
	return home + "/.ssh/known_hosts", nil
}

// ParseHop parses hop from string format.
func ParseHop(s string) (Hop, error) {
	var h Hop
	userPart := ""
	hostPort := s
	if strings.Contains(s, "@") {
		parts := strings.SplitN(s, "@", 2)
		userPart = parts[0]
		hostPort = parts[1]
	}
	if !strings.Contains(hostPort, ":") {
		hostPort += ":22"
	}
	if userPart == "" {
		u, err := user.Current()
		if err != nil || u == nil || u.Username == "" {
			return Hop{}, fmt.Errorf("missing user in hop %q and failed to detect current user: %w", s, err)
		}
		userPart = u.Username
	}
	h.User = userPart
	h.HostPort = hostPort
	return h, nil
}

// ParseHops parses multiple hops from their string representations.
func ParseHops(hops []string) ([]Hop, error) {
	out := make([]Hop, 0, len(hops))
	for _, s := range hops {
		if s == "" {
			continue
		}
		h, err := ParseHop(s)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}
