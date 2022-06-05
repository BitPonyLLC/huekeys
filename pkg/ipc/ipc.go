// Package ipc is a helper for passing Cobra commands across a Unix domain
// socket (UDS).
package ipc

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// IPCServer is the type to set up and listen for clients that want to relay
// commands for the server to execute.
type IPCServer struct {
	ctx           context.Context
	log           *zerolog.Logger
	cmd           *cobra.Command
	listener      net.Listener
	conns         sync.Map
	stopRequested bool
}

// Start will set up and listen on a Unix domain socket (UDS) located at the
// provided path until the context provided is canceled. Any clients that attach
// may issue string commands which will be parsed and processed by the provided
// Cobra command.
func (ipc *IPCServer) Start(ctx context.Context, log *zerolog.Logger, path string, cmd *cobra.Command) error {
	err := os.Remove(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to remove %s: %w", path, err)
		}
	}

	var lc net.ListenConfig
	ipc.listener, err = lc.Listen(ctx, "unix", path)
	if err != nil {
		return fmt.Errorf("unable to listen on %s: %w", path, err)
	}

	// let anyone talk to us
	err = os.Chmod(path, 0666)
	if err != nil {
		return fmt.Errorf("unable to change permissions to %s: %w", path, err)
	}

	ipc.ctx = ctx
	ipc.log = log
	ipc.cmd = cmd

	go func() {
		defer func() {
			util.LogRecover()
			ipc.Stop()
		}()

		for !ipc.stopRequested {
			conn, err := ipc.listener.Accept()
			if err != nil {
				if !ipc.stopRequested {
					ipc.log.Err(err).Str("path", path).Msg("unable to accept new connection")
				}
				return
			}

			ac := &acceptedConn{conn: conn}
			go ac.processCommand(ipc)
		}
	}()

	return nil
}

// Stop will close any client connections outstanding as well as the Unix domain
// socket (UDS) listener.
func (ipc *IPCServer) Stop() error {
	if ipc.listener == nil {
		return nil
	}

	ipc.stopRequested = true

	// cleanup (our context was canceled)
	ipc.conns.Range(func(key, value any) bool {
		ac := key.(*acceptedConn)
		ac.conn.Close()
		return true
	})

	l := ipc.listener
	ipc.listener = nil
	return l.Close()
}
