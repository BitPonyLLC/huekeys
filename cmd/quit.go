package cmd

import (
	"errors"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var quitCmd = &cobra.Command{
	Use:   "quit",
	Short: "Tells remote process to quit",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pidPath.IsRunning() {
			if pidPath.IsOurs() {
				log.Info().Msg("received request to quit")
				cancelFunc()
				return nil
			}

			return sendViaIPC(cmd)
		}

		return errors.New("no remote process found")
	},
}

func init() {
	rootCmd.AddCommand(quitCmd)
}
