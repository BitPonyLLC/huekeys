package cmd

import (
	"os"
	"strings"

	"github.com/BitPonyLLC/huekeys/pkg/ipc"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func sendViaIPC(cmd *cobra.Command) error {
	msg := strings.Join(os.Args[1:], " ")
	log.Debug().Int("pid", pidPath.Getpid()).Str("cmd", msg).Msg("sending")

	resp, err := ipc.Send(viper.GetString("sockpath"), msg)
	if err != nil {
		return err
	}

	if resp != "" {
		cmd.Print(resp)
	}

	return nil
}
