package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// Client is the type used when communicating with an IPCServer.
type Client struct {
	Foreground bool
	RespCB     func(string) bool

	conn       net.Conn
	lastLineAt time.Time
}

// Send is invoked when a caller wants to connect to an IPCServer listening on
// the provided path to issue a command as described by the provided msg.
func Send(path, msg string) (string, error) {
	resp := ""
	client := &Client{
		RespCB: func(s string) bool {
			resp += s
			return true
		},
	}

	err := client.Send(path, msg)
	return resp, err
}

// Send can be invoked on a custom Client with an optional mechanism for
// processing responses and whether to work in the foreground instead of a
// Goroutine.
func (c *Client) Send(path, msg string) error {
	var err error
	c.conn, err = net.Dial("unix", path)
	if err != nil {
		return fmt.Errorf("unable to connect to %s: %w", path, err)
	}
	defer c.conn.Close()

	_, err = c.conn.Write([]byte(msg + "\n"))
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			// server is gone
			return nil
		}

		return fmt.Errorf("unable to send message to %s: %w", path, err)
	}

	if c.Foreground {
		// read until remote closes our connection indicating the command is complete (or accepted)
		c.readResponse()
	} else {
		// listen for any immediate responses
		c.lastLineAt = time.Now()

		go c.readResponse()

		// keep waiting if we're still reading something (e.g. immediate errors
		// need to be reported in case of failure)
		for time.Since(c.lastLineAt) < lastLineIdleDelay {
			time.Sleep(lastLineIdleDelay)
		}
	}

	return nil
}

// Close will terminate the connection.
func (c *Client) Close() {
	c.conn.Close()
}

//--------------------------------------------------------------------------------
// private

const lastLineIdleDelay = 100 * time.Millisecond

func (c *Client) readResponse() {
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		line := scanner.Text()
		c.lastLineAt = time.Now()
		if !c.RespCB(line + "\n") {
			return
		}
	}
}
