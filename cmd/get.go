package cmd

import (
	"fmt"

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
	},
}
