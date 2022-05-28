package ipc

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

type IPCClient struct {
	conn net.Conn
	path string
}

func (ipc *IPCClient) Connect(path string) error {
	var err error
	ipc.conn, err = net.Dial("unix", path)
	if err != nil {
		return fmt.Errorf("unable to connect to %s: %w", path, err)
	}

	ipc.path = path
	return nil
}

func (ipc *IPCClient) Send(msg string) (string, error) {
	if ipc.conn == nil {
		err := ipc.Connect(ipc.path)
		if err != nil {
			return "", err
		}
	}

	_, err := ipc.conn.Write([]byte(msg + "\n"))
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			ipc.conn = nil // server is gone: attempt reconnect on next message
		}

		return "", fmt.Errorf("unable to send message to %s: %w", ipc.path, err)
	}

	// listen for any immediate responses
	lastLineAt := time.Now()
	resp := ""
	go func() {
		sep := ""
		scanner := bufio.NewScanner(ipc.conn)
		for scanner.Scan() {
			line := scanner.Text()
			resp += sep + line
			sep = "\n"
			lastLineAt = time.Now()
		}
	}()

	// keep waiting if we're still reading something
	for time.Since(lastLineAt) < 100*time.Millisecond {
		time.Sleep(100 * time.Millisecond)
	}

	return resp, nil
}

func (ipc *IPCClient) Close() error {
	return ipc.conn.Close()
}
