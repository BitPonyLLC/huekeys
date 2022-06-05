package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

const lastLineIdleDelay = 100 * time.Millisecond

// Send is invoked when a caller wants to connect to an IPCServer listening on
// the provided path to issue a command as described by the provided msg.
func Send(path, msg string) (string, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return "", fmt.Errorf("unable to connect to %s: %w", path, err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(msg + "\n"))
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			conn = nil // server is gone: attempt reconnect on next message
		}

		return "", fmt.Errorf("unable to send message to %s: %w", path, err)
	}

	// listen for any immediate responses
	lastLineAt := time.Now()
	resp := ""
	go func() {
		sep := ""
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			lastLineAt = time.Now()
			resp += sep + line
			sep = "\n"
		}
	}()

	// keep waiting if we're still reading something
	for time.Since(lastLineAt) < lastLineIdleDelay {
		time.Sleep(lastLineIdleDelay)
	}

	return resp, nil
}
