package cmd

import (
	"github.com/BitPonyLLC/huekeys/pkg/keyboard"

	"github.com/spf13/cobra"
)

// Listcolors indicates when to simply list out all color names available
var ListColors bool

// Color represents keyboard color pattern to run
var Color string

// Brightness represents the value to set for keyboard brightness
var Brightness string

func init() {
	rootCmd.AddCommand(setCmd)
	setCmd.Flags().BoolVar(&ListColors, "list", false, "lists out all color names and values")
	setCmd.Flags().StringVarP(&Color, "color", "c", "", "sets the color using a name, hex value, or \"random\"")
	setCmd.Flags().StringVarP(&Brightness, "brightness", "b", "", "sets the backlight brightness (0 - 255)")
}

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Sets the color and brightness of the keyboard",
	Long:  `Sets the color and brightness of the keyboard`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if ListColors {
			keyboard.EachPresetColor(func(name, value string) {
				cmd.Printf("%s = %s\n", name, value)
			})
		}

		if Color != "" {
			err := keyboard.ColorFileHandler(Color)
			if err != nil {
				return fail(11, err)
			}
		}

		if Brightness != "" {
			err := keyboard.BrightnessFileHandler(Brightness)
			if err != nil {
				return fail(12, err)
			}
		}

		return nil
	},
}
