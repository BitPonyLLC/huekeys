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

type IPCServer struct {
	LastCmd string

	ctx           context.Context
	log           *zerolog.Logger
	cmd           *cobra.Command
	listener      net.Listener
	conns         sync.Map
	stopRequested bool
}

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
					ipc.log.Error().Err(err).Str("path", path).Msg("unable to accept new connection")
				}
				return
			}

			ac := &acceptedConn{conn: conn}
			go ac.handleCommands(ipc)
		}
	}()

	return nil
}

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
