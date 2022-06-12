package cmd

import (
	"os"
	"strings"

	"github.com/BitPonyLLC/huekeys/pkg/ipc"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func sendViaIPC(cmd *cobra.Command) error {
	return sendViaIPCForeground(cmd, false, "")
}

func sendMsgViaIPC(cmd *cobra.Command, msg string) error {
	return sendViaIPCForeground(cmd, false, msg)
}

func sendViaIPCForeground(cmd *cobra.Command, foreground bool, msg string) error {
	if msg == "" {
		msg = strings.Join(os.Args[1:], " ")
	}

	log.Debug().Int("pid", waitPidPath.Getpid()).Str("cmd", msg).Msg("sending")

	client := &ipc.Client{
		Foreground: foreground,
		RespCB: func(line string) bool {
			cmd.Print(line)
			return true
		},
	}

	if foreground {
		go func() {
			<-cmd.Context().Done()
			client.Close()
		}()
	}

	err := client.Send(waitSockPath(), msg)
	if err != nil {
		return err
	}

	return nil
}
