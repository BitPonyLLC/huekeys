package ipc

import (
	"bufio"
	"net"
	"strings"

	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/mattn/go-shellwords"
)

type acceptedConn struct {
	conn net.Conn
}

func (ac *acceptedConn) processCommand(parent *IPCServer) {
	parent.conns.Store(ac, ac)
	defer func() {
		util.LogRecover()
		parent.conns.Delete(ac)
		ac.conn.Close()
		parent.log.Trace().Msg("client disconnected")
	}()

	parent.log.Trace().Msg("client connected")

	outWriter := &ConnWriter{conn: ac.conn}
	errWriter := &ConnWriter{conn: ac.conn, prefix: "ERR: "}
	parent.cmd.SetOut(outWriter)
	parent.cmd.SetErr(errWriter)
	for _, c := range parent.cmd.Commands() {
		c.SetOut(outWriter)
		c.SetErr(errWriter)
	}

	reader := bufio.NewReader(ac.conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		parent.log.Err(err).Msg("unable to read command from client")
		return
	}

	line = strings.TrimSpace(line)
	clog := parent.log.With().Str("cmd", line).Logger()

	args, err := shellwords.Parse(line)
	if err != nil {
		errWriter.Writeln("unable to parse command: %s", line)
	} else {
		clog.Debug().Msg("executing")
		parent.cmd.SetArgs(args)
		err = parent.cmd.ExecuteContext(parent.ctx)
		if err != nil {
			clog.Err(err).Msg("command failed")
		}
	}

	if outWriter.err != nil {
		clog.Err(outWriter.err).Msg("output writer failed")
	}

	if errWriter.err != nil {
		clog.Err(errWriter.err).Msg("error writer failed")
	}
}
