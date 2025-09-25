// Package main shows how you can use an SSH key rather than SSH Agent.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/borud/tunnel"
	"golang.org/x/crypto/ssh"
)

func main() {
	var (
		keyPath = flag.String("key", "", "path to private key file (optional). If omitted, SSH agent is used.")
	)
	flag.Parse()

	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <user@host:port> <url>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}

	hop := flag.Arg(0)
	urlStr := flag.Arg(1)

	// Create the tunnel
	t, err := tunnel.Create(
		tunnel.WithHop(hop),
		tunnel.WithKeyFile(*keyPath, nil), // supposes key file that is not password protected
		tunnel.WithoutAgent(),             // explicitly disallow SSH Agent
		tunnel.WithHostKeyCallback(ssh.InsecureIgnoreHostKey()),
	)
	if err != nil {
		log.Fatalf("tunnel create: %v", err)
	}
	defer t.Close()

	// HTTP client that dials via tunnel
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = t.DialContext

	client := &http.Client{Transport: tr}

	resp, err := client.Get(urlStr)
	if err != nil {
		log.Fatalf("http get: %v", err)
	}
	defer resp.Body.Close()

	n, err := io.Copy(io.Discard, resp.Body) // discard body; change to os.Stdout if you want output
	if err != nil {
		log.Fatalf("copy body: %v", err)
	}
	fmt.Fprintf(os.Stderr, "\n-- %d bytes read --\n", n)
}
