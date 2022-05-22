package cmd

import (
	"github.com/bambash/sys76-kb/internal/image_matcher"
	"github.com/bambash/sys76-kb/pkg/keyboard"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Gets the color and brightness of the keyboard",
	Long:  `Gets the color and brightness of the keyboard`,
	RunE: func(cmd *cobra.Command, args []string) error {
		brightness, err := keyboard.GetCurrentBrightness()
		if err != nil {
			return fail(11, err)
		}
		cmd.Println("brightness =", brightness)
		colors, err := keyboard.GetCurrentColors()
		if err != nil {
			return fail(12, err)
		}
		for key, color := range colors {
			cmd.Printf("%s = %s\n", key, color)
		}
		for _, arg := range args {
			color, err := image_matcher.GetDominantColorOf(arg)
			if err != nil {
				return fail(13, "can't determine dominant color of %s: %w", arg, err)
			}
			cmd.Printf("%s = %s\n", arg, color)
		}
		return nil
	},
}
