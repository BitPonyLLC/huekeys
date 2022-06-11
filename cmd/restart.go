package cmd

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/BitPonyLLC/huekeys/pkg/pidpath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Tells remote process to restart",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !pidPath.IsRunning() {
			return errors.New("no remote process found")
		}

		if pidPath.IsOurs() {
			log.Info().Msg("received request to restart")
			cancelFunc()
			return nil
		}

		menuPidpath = pidpath.NewPidPath(viper.GetString("menu.pidpath"), 0666)
		if !menuPidpath.IsRunning() {
			return sendMsgViaIPC(cmd, "quit")
		}

		log.Info().Str("menu", menuPidpath.String()).Msg("sending restart signal to menu")
		err := syscall.Kill(menuPidpath.Getpid(), syscall.SIGHUP)
		if err != nil {
			return fmt.Errorf("unable to kill menu process: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
