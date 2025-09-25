package tunnel

import (
	"net"
	"sync"
)

type trackedConn struct {
	net.Conn
	onClose func()
	once    sync.Once
}

func (c *trackedConn) Close() error {
	var err error
	c.once.Do(func() {
		err = c.Conn.Close()
		if c.onClose != nil {
			c.onClose()
		}
	})
	return err
}
