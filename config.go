package tunnel

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config contains the configuration for the tunnel.
type Config struct {
	Hops           []Hop
	Signers        []ssh.Signer
	UseAgent       bool
	KnownHostsPath string
	HostKeyCB      ssh.HostKeyCallback // global override (lowest priority: hop > global > path)
	PerHopTimeout  time.Duration
	KeepAlive      time.Duration
	TrackConns     bool
	Logger         *slog.Logger
}

// Option is a configuration option
type Option func(*Config) error

func defaultConfig() Config {
	return Config{
		KnownHostsPath: "",
		PerHopTimeout:  10 * time.Second,
		KeepAlive:      30 * time.Second,
		TrackConns:     true,
		Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
		})).With("component", "tunnel"),
	}
}

// WithHop adds a single hop in "user@host:port" form. If port is omitted,
// defaults to :22. If user is omitted, the current user is used.
func WithHop(s string) Option {
	return func(c *Config) error {
		h, err := ParseHop(s)
		if err != nil {
			return err
		}
		c.Hops = append(c.Hops, h)
		return nil
	}
}

// WithHops adds multiple pre-parsed Hop objects to the configuration.
func WithHops(hops ...Hop) Option {
	return func(c *Config) error {
		c.Hops = append(c.Hops, hops...)
		return nil
	}
}

// WithSigner adds an in-memory ssh.Signer (private key) to be used for authentication.
func WithSigner(s ssh.Signer) Option {
	return func(c *Config) error {
		c.Signers = append(c.Signers, s)
		return nil
	}
}

// WithKey parses a private key from in-memory PEM data and adds it as an ssh.Signer.
// If passphrase is non-nil, it is used to decrypt the key.
func WithKey(pemBytes []byte, passphrase []byte) Option {
	return func(c *Config) error {
		var s ssh.Signer
		var err error
		if len(passphrase) > 0 {
			s, err = ssh.ParsePrivateKeyWithPassphrase(pemBytes, passphrase)
		} else {
			s, err = ssh.ParsePrivateKey(pemBytes)
		}
		if err != nil {
			return fmt.Errorf("parse key: %w", err)
		}
		c.Signers = append(c.Signers, s)
		return nil
	}
}

// WithKeyFile loads a private key from a PEM file on disk and adds it as an ssh.Signer.
// If passphrase is non-nil, it is used to decrypt the key.
func WithKeyFile(path string, passphrase []byte) Option {
	return func(c *Config) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read key %q: %w", path, err)
		}
		var s ssh.Signer
		if len(passphrase) > 0 {
			s, err = ssh.ParsePrivateKeyWithPassphrase(b, passphrase)
		} else {
			s, err = ssh.ParsePrivateKey(b)
		}
		if err != nil {
			return fmt.Errorf("parse key %q: %w", path, err)
		}
		c.Signers = append(c.Signers, s)
		return nil
	}
}

// WithAgent enables using the SSH agent for authentication, if SSH_AUTH_SOCK is set.
// If no agent is available, no agent auth method will be added.
func WithAgent() Option {
	return func(c *Config) error {
		c.UseAgent = true
		return nil
	}
}

// WithoutAgent disables SSH agent usage, even if SSH_AUTH_SOCK is set.
func WithoutAgent() Option {
	return func(c *Config) error {
		c.UseAgent = false
		return nil
	}
}

// WithKnownHosts sets the path to a known_hosts file for host key verification.
// If not provided, defaults to ~/.ssh/known_hosts.
func WithKnownHosts(path string) Option {
	return func(c *Config) error {
		c.KnownHostsPath = path
		return nil
	}
}

// WithHostKeyCallback sets a custom ssh.HostKeyCallback for host key verification.
// This overrides known_hosts file configuration.
func WithHostKeyCallback(cb ssh.HostKeyCallback) Option {
	return func(c *Config) error {
		c.HostKeyCB = cb
		return nil
	}
}

// WithPerHopTimeout sets the timeout used when dialing each SSH hop.
// Defaults to 10 seconds.
func WithPerHopTimeout(d time.Duration) Option {
	return func(c *Config) error {
		c.PerHopTimeout = d
		return nil
	}
}

// WithKeepAlive sets the interval for sending SSH keep-alive requests to each hop.
// Use 0 to disable keep-alives. Default is 30 seconds.
func WithKeepAlive(d time.Duration) Option {
	return func(c *Config) error {
		c.KeepAlive = d
		return nil
	}
}

// WithConnTracking enables or disables connection tracking.
// When enabled, Tunnel.Close() will also close any Conns or Listeners created by the tunnel.
func WithConnTracking(enable bool) Option {
	return func(c *Config) error {
		c.TrackConns = enable
		return nil
	}
}

// WithLogger replaces the default slog.Logger with a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *Config) error {
		c.Logger = l
		return nil
	}
}
