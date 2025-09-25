package tunnel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestTunnelDialContextDirectTCPIP(t *testing.T) {
	// 1) Start a local TCP echo server (the final target we'll connect to over the SSH tunnel).
	echoLn, echoAddr := startTCPEcho(t)

	// 2) Start an in-process SSH server that accepts public key auth and supports direct-tcpip.
	sshd, sshAddr := startSSHServer(t)

	// 3) Generate a client keypair & signer (no passphrase) and build the tunnel with one hop.
	_, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientPriv)
	if err != nil {
		t.Fatalf("ssh.NewSignerFromKey: %v", err)
	}

	tun, err := Create(
		WithHop("testuser@"+sshAddr),
		WithSigner(clientSigner),
		// Use insecure host-key checking for the test server (we generate a random hostkey each run).
		WithHostKeyCallback(ssh.InsecureIgnoreHostKey()),
		WithPerHopTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer tun.Close()

	// 4) Use the tunnel to Dial the echo server and verify echo behavior.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := tun.DialContext(ctx, "tcp", echoAddr)
	if err != nil {
		t.Fatalf("DialContext via tunnel: %v", err)
	}
	defer conn.Close()

	payload := []byte("hello over ssh tunnel")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("readfull: %v", err)
	}
	if string(buf) != string(payload) {
		t.Fatalf("echo mismatch: got %q want %q", buf, payload)
	}

	// Cleanup listeners.
	_ = echoLn.Close()
	_ = sshd.Close()
}

func startTCPEcho(t *testing.T) (net.Listener, string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo listen: %v", err)
	}
	addr := ln.Addr().String()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				io.Copy(conn, conn)
			}(c)
		}
	}()
	return ln, addr
}

type sshTestServer struct {
	ln   net.Listener
	done chan struct{}
	once sync.Once
}

func (s *sshTestServer) Close() error {
	s.once.Do(func() {
		close(s.done)
		_ = s.ln.Close()
	})
	return nil
}

func startSSHServer(t *testing.T) (*sshTestServer, string) {
	// Generate server host key (ed25519).
	_, srvPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("server ed25519.GenerateKey: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(srvPriv)
	if err != nil {
		t.Fatalf("ssh.NewSignerFromKey(host): %v", err)
	}

	// Accept any public key (sufficient for tests).
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	cfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ssh listen: %v", err)
	}

	s := &sshTestServer{ln: ln, done: make(chan struct{})}
	addr := ln.Addr().String()

	go func() {
		for {
			nc, err := ln.Accept()
			if err != nil {
				return
			}
			go handleSSHConn(nc, cfg)
		}
	}()

	return s, addr
}

func handleSSHConn(nc net.Conn, cfg *ssh.ServerConfig) {
	defer nc.Close()
	sconn, chans, reqs, err := ssh.NewServerConn(nc, cfg)
	if err != nil {
		return
	}
	defer sconn.Close()

	// Handle global requests (reply to keepalive@openssh.com).
	go func() {
		for req := range reqs {
			if req.WantReply {
				_ = req.Reply(true, nil)
			}
		}
	}()

	// Handle channels: support "direct-tcpip" to act as a simple TCP forwarder.
	for newCh := range chans {
		if newCh.ChannelType() != "direct-tcpip" {
			newCh.Reject(ssh.UnknownChannelType, "unsupported channel")
			continue
		}
		go handleDirectTCPIP(newCh)
	}
}

type directTCPIPReq struct {
	DestAddr string
	DestPort uint32
	OrigAddr string
	OrigPort uint32
}

func handleDirectTCPIP(newCh ssh.NewChannel) {
	var req directTCPIPReq
	if err := ssh.Unmarshal(newCh.ExtraData(), &req); err != nil {
		newCh.Reject(ssh.ConnectionFailed, "bad direct-tcpip payload")
		return
	}
	target := fmt.Sprintf("%v:%d", req.DestAddr, req.DestPort)
	backend, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		newCh.Reject(ssh.ConnectionFailed, "dial backend failed")
		return
	}
	ch, reqs, err := newCh.Accept()
	if err != nil {
		_ = backend.Close()
		return
	}
	go ssh.DiscardRequests(reqs)

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		io.Copy(ch, backend)
		_ = ch.CloseWrite()
		wg.Done()
	}()
	go func() {
		io.Copy(backend, ch)
		_ = backend.(*net.TCPConn).CloseWrite()
		wg.Done()
	}()
	wg.Wait()
	ch.Close()
	backend.Close()
}
