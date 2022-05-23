package cmd

import (
	"github.com/BitPonyLLC/huekeys/pkg/keyboard"

	"github.com/spf13/cobra"
)

var listColors bool
var color string
var brightness string

func init() {
	rootCmd.AddCommand(setCmd)
	setCmd.Flags().BoolVar(&listColors, "list", false, "lists out all color names and values")
	setCmd.Flags().StringVarP(&color, "color", "c", "", "sets the color using a name, hex value, or \"random\"")
	setCmd.Flags().StringVarP(&brightness, "brightness", "b", "", "sets the backlight brightness (0 - 255)")
}

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Sets the color and/or brightness of the keyboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		showHelp := true

		if listColors {
			showHelp = false
			keyboard.EachPresetColor(func(name, value string) {
				cmd.Printf("%s = %s\n", name, value)
			})
		}

		if color != "" {
			showHelp = false
			err := keyboard.ColorFileHandler(color)
			if err != nil {
				return fail(11, err)
			}
		}

		if brightness != "" {
			showHelp = false
			err := keyboard.BrightnessFileHandler(brightness)
			if err != nil {
				return fail(12, err)
			}
		}

		if showHelp {
			cmd.Help()
			cmd.Println()
			return fail(13, "set requires one or more flags")
		}

		return nil
	},
}
