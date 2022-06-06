package ipc

import (
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
)

// ConnWriter is an io.Writer that will relay any bytes written to it into the
// associated connection.
type ConnWriter struct {
	conn   net.Conn
	prefix string
	err    error
}

var _ io.Writer = (*ConnWriter)(nil) // ensures we conform to the io.Writer interface

// Write will write bytes to the connection.
func (cw *ConnWriter) Write(p []byte) (int, error) {
	p = append([]byte(cw.prefix), p...)
	n, err := cw.conn.Write(p)
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			// client is gone: don't write this to errors, but do pass it along to caller
			return n, err
		}

		cw.err = err
	}

	return n, err
}

// Writeln will write a formatted message to the connection.
func (cw *ConnWriter) Writeln(format string, args ...any) {
	cw.Write([]byte(fmt.Sprintf(format+"\n", args...)))
}
