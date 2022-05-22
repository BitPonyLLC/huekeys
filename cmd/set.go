package cmd

import (
	"fmt"
	"os"

	keyboard "github.com/bambash/sys76-kb/pkg"
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
	Run: func(cmd *cobra.Command, args []string) {
		if ListColors {
			keyboard.EachPresetColor(func(name, value string) {
				fmt.Printf("%s = %s\n", name, value)
			})
		}
		if Color != "" {
			err := keyboard.ColorFileHandler(Color)
			if err != nil {
				cmd.PrintErrln(err)
				os.Exit(1)
			}
		}
		if Brightness != "" {
			keyboard.BrightnessFileHandler(Brightness)
		}
	},
}
