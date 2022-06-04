package cmd

import (
	"errors"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
	"github.com/BitPonyLLC/huekeys/pkg/patterns"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var off bool

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Tells remote process to stop any running pattern",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if pidPath.IsRunning() {
			if pidPath.IsOurs() {
				if off {
					log.Info().Msg("received request to turn keyboard off")
					err = keyboard.BrightnessFileHandler("0")
				}

				running := patterns.GetRunning()
				if running == nil {
					log.Info().Msg("received request to stop with nothing running")
					return
				}

				log.Info().Str("pattern", running.GetBase().Name).Msg("received request to stop")
				running.Stop()
				return
			}

			return sendViaIPC(cmd)
		}

		return errors.New("no remote process found")
	},
}

func init() {
	stopCmd.Flags().BoolVar(&off, "off", off, "also turn the keyboard lights off")
	rootCmd.AddCommand(stopCmd)
}
