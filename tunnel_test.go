package tunnel

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestCreateNoHops(t *testing.T) {
	_, err := Create(
		// No hops at all
		WithAgent(), // auth present, but should still fail due to no hops
	)
	if err == nil || err.Error() == "" {
		t.Fatalf("Create without hops: expected error, got nil")
	}
	if err != ErrNoHops {
		t.Fatalf("Create without hops: want ErrNoHops, got %v", err)
	}
}

func TestCreateNoAuth(t *testing.T) {
	_, err := Create(
		WithHop("alice@host:22"),
		// no signer, no agent
	)
	if err == nil {
		t.Fatalf("Create without auth: expected error, got nil")
	}
	if errUnwrap := unwrap(err); errUnwrap != ErrNoAuth {
		t.Fatalf("Create without auth: want ErrNoAuth, got %v", err)
	}
}

func TestCreateWithAgentOnlySucceeds(t *testing.T) {
	// WithAgent() is explicit; Create should succeed even if SSH_AUTH_SOCK
	// is not set — dialing isn't attempted until ensureChain().
	_, err := Create(
		WithHop("alice@host:22"),
		WithAgent(),
	)
	if err != nil {
		t.Fatalf("Create with agent only: unexpected error: %v", err)
	}
}

func TestCreateWithSignerSucceeds(t *testing.T) {
	// Generate an in-memory ed25519 key and wrap as ssh.Signer
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	_, err = Create(
		WithHop("alice@host:22"),
		WithSigner(signer),
	)
	if err != nil {
		t.Fatalf("Create with signer: unexpected error: %v", err)
	}
}

// unwrap extracts the sentinel from error chains for comparisons in tests.
func unwrap(err error) error {
	// We just traverse using errors.Is to compare sentinels.
	// Return the sentinel we matched (or the original if none matched).
	if err == nil {
		return nil
	}
	if is(err, ErrNoAuth) {
		return ErrNoAuth
	}
	if is(err, ErrNoHops) {
		return ErrNoHops
	}
	if is(err, ErrClosed) {
		return ErrClosed
	}
	return err
}

func is(err, target error) bool { return target != nil && errorIs(err, target) }

// errorIs is a tiny indirection so the tests still compile on Go 1.20+
// without importing "errors" in multiple places; adjust to your preference.
func errorIs(err, target error) bool {
	// stdlib errors.Is
	type causer interface{ Unwrap() error }
	for {
		if err == target {
			return true
		}
		c, ok := err.(causer)
		if !ok {
			return false
		}
		err = c.Unwrap()
	}
}
