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
	Ctx  context.Context
	Path string
	Cmd  *cobra.Command
	Log  *zerolog.Logger

	conns sync.Map
}

func (ipc *IPCServer) Start() error {
	err := os.Remove(ipc.Path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to remove %s: %w", ipc.Path, err)
		}
	}

	var lc net.ListenConfig
	l, err := lc.Listen(ipc.Ctx, "unix", ipc.Path)
	if err != nil {
		return fmt.Errorf("unable to listen on %s: %w", ipc.Path, err)
	}

	// let anyone talk to us
	err = os.Chmod(ipc.Path, 0666)
	if err != nil {
		return fmt.Errorf("unable to change permissions to %s: %w", ipc.Path, err)
	}

	go func() {
		defer func() {
			util.LogRecover()
			l.Close()
		}()

		for {
			conn, err := l.Accept()
			if err != nil {
				ipc.Log.Error().Err(err).Str("path", ipc.Path).Msg("unable to accept new connection")
				// FIXME: probably want to continue here for some kinds of errors
				break
			}

			ac := &acceptedConn{
				parent: ipc,
				conn:   conn,
				cmd:    *ipc.Cmd, // COPY the original command to allow multiple client usages
			}

			go ac.handleCommands()
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
