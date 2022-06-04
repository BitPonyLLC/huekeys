package cmd

import (
	"errors"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Tells remote process to pause any running pattern",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pidPath.IsRunning() {
			if pidPath.IsOurs() {
				log.Info().Msg("received request to pause")
				return nil
			}

			return sendViaIPC(cmd)
		}

		return errors.New("no remote process found")
	},
}

func init() {
	rootCmd.AddCommand(pauseCmd)
}
