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
	ctx   context.Context
	log   *zerolog.Logger
	conns sync.Map
	cmd   *cobra.Command
}

func (ipc *IPCServer) Start(ctx context.Context, log *zerolog.Logger, path string, cmd *cobra.Command) error {
	err := os.Remove(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to remove %s: %w", path, err)
		}
	}

	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "unix", path)
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
			l.Close()
		}()

		for {
			conn, err := l.Accept()
			if err != nil {
				ipc.log.Error().Err(err).Str("path", path).Msg("unable to accept new connection")
				// FIXME: probably want to continue here for some kinds of errors
				break
			}

			ac := &acceptedConn{conn: conn}
			go ac.handleCommands(ipc)
		}

		// cleanup (our context was canceled)
		ipc.conns.Range(func(key, value any) bool {
			ac := key.(*acceptedConn)
			ac.conn.Close()
			return true
		})
	}()

	return nil
}
