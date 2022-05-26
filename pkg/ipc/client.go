package ipc

import (
	"bufio"
	"fmt"
	"net"
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
	_, err := ipc.conn.Write([]byte(msg + "\n"))
	if err != nil {
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
