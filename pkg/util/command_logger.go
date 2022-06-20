package util

import (
	"bytes"
	"io"
	"sync"
)

type CommandLogger struct {
	Log func(string)

	buf   bytes.Buffer
	mutex sync.Mutex
}

var _ io.WriteCloser = (*CommandLogger)(nil) // ensures we conform to the WriteCloser interface

func (cl *CommandLogger) Write(data []byte) (n int, err error) {
	cl.mutex.Lock()
	defer cl.mutex.Unlock()

	n, err = cl.buf.Write(data)
	if err != nil {
		return
	}

	for {
		var line *string
		line, err = cl.getLine()
		if err != nil {
			return
		}

		if line == nil {
			return // no line found in the buffer yet...
		}

		if *line != "" {
			cl.Log(*line)
		}
	}
}

func (cl *CommandLogger) Close() error {
	if cl.buf.Len() > 0 {
		cl.Log(cl.buf.String()) // flush
	}

	cl.buf.Truncate(0)
	return nil
}

//--------------------------------------------------------------------------------
// private

// using our own instead of bufio.NewScanner as that will return all the bytes,
// but we want to wait for more writes to get the next newline
func (cl *CommandLogger) getLine() (*string, error) {
	i := bytes.IndexRune(cl.buf.Bytes(), '\n')
	if i < 0 {
		return nil, nil
	}

	i++ // length is offset plus one

	buf := make([]byte, i)
	n, err := cl.buf.Read(buf)
	if err != nil {
		return nil, err
	}

	if n != i {
		return nil, io.ErrShortBuffer
	}

	str := string(buf[0 : i-1]) // do not include the newline
	return &str, nil
}
