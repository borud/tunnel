// Package main implements a simple example of a tunneled HTTP client.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/borud/tunnel"
	"golang.org/x/crypto/ssh"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("\n%s <user@host:port> <url>\n", os.Args[0])
		return
	}

	hop := os.Args[1]
	urlStr := os.Args[2]

	// create the tunnel
	t, err := tunnel.Create(
		tunnel.WithHop(hop), // add a single hop
		tunnel.WithAgent(),  // we want to use the SSH Agent for authentication
		tunnel.WithHostKeyCallback(ssh.InsecureIgnoreHostKey()), // we skip host key checking
	)
	if err != nil {
		log.Fatalf("tunnel create: %v", err)
	}
	defer t.Close()

	// Wrap http.Transport so it dials through the tunnel
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = t.DialContext

	client := &http.Client{Transport: transport}

	resp, err := client.Get(urlStr)
	if err != nil {
		log.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()

	// just discard the HTTP body
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		log.Fatalf("copy body: %v", err)
	}
	fmt.Fprintf(os.Stderr, "\n-- %d bytes read --\n", n)
}
