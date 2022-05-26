package ipc

import (
	"bufio"
	"net"

	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/mattn/go-shellwords"
)

type acceptedConn struct {
	conn net.Conn
}

func (ac *acceptedConn) handleCommands(parent *IPCServer) {
	parent.conns.Store(ac, ac)
	defer func() {
		util.LogRecover()
		parent.conns.Delete(ac)
		ac.conn.Close()
		parent.log.Info().Msg("client disconnected")
	}()

	parent.log.Info().Msg("client connected")

	outWriter := &ConnWriter{conn: ac.conn}
	errWriter := &ConnWriter{conn: ac.conn, prefix: "ERR: "}
	parent.cmd.SetOut(outWriter)
	parent.cmd.SetErr(errWriter)
	for _, c := range parent.cmd.Commands() {
		c.SetOut(ac.conn)
		c.SetErr(ac.conn)
	}

	scanner := bufio.NewScanner(ac.conn)
	for scanner.Scan() {
		line := scanner.Text()
		clog := parent.log.With().Str("cmd", line).Logger()

		args, err := shellwords.Parse(line)
		if err != nil {
			errWriter.Writeln("unable to parse command: %s", line)
		} else {
			// need to run async to allow more commands from client
			go func() {
				defer util.LogRecover()
				clog.Debug().Msg("executing")
				parent.cmd.SetArgs(args)
				err = parent.cmd.ExecuteContext(parent.ctx)
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
