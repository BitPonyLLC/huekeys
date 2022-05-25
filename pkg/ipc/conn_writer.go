package ipc

import (
	"fmt"
	"io"
	"net"
)

type ConnWriter struct {
	conn   net.Conn
	prefix string
	err    error
}

var _ io.Writer = (*ConnWriter)(nil) // ensures we conform to the io.Writer interface

func (cw *ConnWriter) Write(p []byte) (int, error) {
	p = append([]byte(cw.prefix), p...)
	n, err := cw.conn.Write(p)
	if err != nil {
		cw.err = err
	}
	return n, err
}

func (cw *ConnWriter) Writeln(format string, args ...any) {
	cw.Write([]byte(fmt.Sprintf(format+"\n", args...)))
}
