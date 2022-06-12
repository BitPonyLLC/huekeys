package cmd

import (
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
		if !waitPidPath.IsRunning() {
			// don't fail here as this is invoked by post_install
			return fail(0, "no remote process found")
		}

		if waitPidPath.IsOurs() {
			log.Info().Msg("received request to restart")
			cancelFunc()
			return nil
		}

		menuPidPath = pidpath.NewPidPath(viper.GetString("menu.pidpath"), 0666)
		if !menuPidPath.IsRunning() {
			return sendMsgViaIPC(cmd, "quit")
		}

		log.Info().Str("menu", menuPidPath.String()).Msg("sending restart signal to menu")
		err := syscall.Kill(menuPidPath.Getpid(), syscall.SIGHUP)
		if err != nil {
			return fmt.Errorf("unable to kill menu process: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
