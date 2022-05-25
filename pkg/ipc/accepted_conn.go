package ipc

import (
	"bufio"
	"net"

	"github.com/BitPonyLLC/huekeys/pkg/util"
	"github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
)

type acceptedConn struct {
	parent *IPCServer
	conn   net.Conn
	cmd    cobra.Command
}

func (ac *acceptedConn) handleCommands() {
	ac.parent.conns.Store(ac, ac)
	defer func() {
		util.LogRecover()
		ac.parent.conns.Delete(ac)
		ac.conn.Close()
		ac.parent.Log.Info().Msg("client disconnected")
	}()

	ac.parent.Log.Info().Msg("client connected")

	outWriter := &ConnWriter{conn: ac.conn}
	errWriter := &ConnWriter{conn: ac.conn, prefix: "ERR: "}
	ac.cmd.SetOut(outWriter)
	ac.cmd.SetErr(errWriter)
	for _, c := range ac.cmd.Commands() {
		c.SetOut(ac.conn)
		c.SetErr(ac.conn)
	}

	scanner := bufio.NewScanner(ac.conn)
	for scanner.Scan() {
		line := scanner.Text()
		clog := ac.parent.Log.With().Str("cmd", line).Logger()

		args, err := shellwords.Parse(line)
		if err != nil {
			errWriter.Writeln("unable to parse command: %s", line)
		} else {
			// need to run async to allow more commands from client
			go func() {
				defer util.LogRecover()
				clog.Debug().Msg("executing")
				ac.cmd.SetArgs(args)
				err = ac.cmd.ExecuteContext(ac.parent.Ctx)
				if err != nil {
					clog.Error().Err(err).Msg("command failed")
				}
			}()
		}

		done := false

		if outWriter.err != nil {
			done = true
			clog.Error().Err(outWriter.err).Msg("output writer failed")
		}

		if errWriter.err != nil {
			done = true
			clog.Error().Err(errWriter.err).Msg("error writer failed")
		}

		if done {
			break
		}
	}
}
