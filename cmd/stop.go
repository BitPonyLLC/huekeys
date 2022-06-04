package cmd

import (
	"errors"

	"github.com/BitPonyLLC/huekeys/pkg/patterns"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "stop",
	Short: "Tells remote process to stop any running pattern",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pidPath.IsRunning() {
			if pidPath.IsOurs() {
				running := patterns.GetRunning()
				if running == nil {
					log.Info().Msg("received request to stop with nothing running")
					return nil
				}

				log.Info().Str("pattern", running.GetBase().Name).Msg("received request to stop")
				running.Stop()
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
