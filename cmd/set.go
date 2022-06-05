package cmd

import (
	"strconv"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"

	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set { list | <color-name> | <color-hex-code> | <brightness-number> }...",
	Short: "Sets the color and/or brightness of the keyboard",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if pidPath.IsRunning() && !pidPath.IsOurs() {
			return sendViaIPC(cmd)
		}

		for _, arg := range args {
			if arg == "list" {
				keyboard.EachPresetColor(func(name, value string) {
					cmd.Printf("%s = %s\n", name, value)
				})

				continue
			}

			val, err := strconv.Atoi(arg)
			if err == nil && val < 256 {
				err := keyboard.BrightnessFileHandler(arg)
				if err != nil {
					return fail(12, err)
				}

				continue
			}

			err = keyboard.ColorFileHandler(arg)
			if err != nil {
				return fail(11, err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(setCmd)
}
