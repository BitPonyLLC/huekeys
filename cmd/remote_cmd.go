package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BitPonyLLC/huekeys/buildinfo"
	"github.com/BitPonyLLC/huekeys/pkg/ipc"
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"
)

var sockPath = filepath.Join(os.TempDir(), buildinfo.Name+".sock")

func sendViaIPC(cmd *cobra.Command) error {
	log.Debug().Int("pid", pidPath.Getpid()).Msg("sending command to running process")

	cli := &ipc.IPCClient{}
	err := cli.Connect(sockPath)
	if err != nil {
		return err
	}
	defer cli.Close()

	msg := strings.Join(os.Args[1:], " ")
	resp, err := cli.Send(msg)
	if err != nil {
		return err
	}

	if resp != "" {
		cmd.Print(resp)
	}

	return nil
}
