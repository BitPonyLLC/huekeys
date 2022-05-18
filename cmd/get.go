package cmd

import (
	"fmt"
	"os"

	"github.com/bambash/sys76-kb/internal/image_matcher"
	keyboard "github.com/bambash/sys76-kb/pkg"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(getCmd)
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Gets the color and brightness of the keyboard",
	Long:  `Gets the color and brightness of the keyboard`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("brightness =", keyboard.GetCurrentBrightness())
		for key, color := range keyboard.GetCurrentColors() {
			fmt.Printf("%s = %s\n", key, color)
		}
		for _, arg := range args {
			color, err := image_matcher.GetDominantColorOf(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "can't determine dominant color of %s: %v\n", arg, err)
				os.Exit(1)
			}
			fmt.Printf("%s = %s\n", arg, color)
		}
	},
}
