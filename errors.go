package tunnel

import "errors"

// errors
var (
	ErrNoHops = errors.New("no hops configured")
	ErrNoAuth = errors.New("no SSH auth methods configured")
	ErrClosed = errors.New("tunnel closed")
)
