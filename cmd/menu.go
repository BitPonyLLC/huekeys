package cmd

import (
	"strings"

	"github.com/BitPonyLLC/huekeys/internal/menu"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(menuCmd)
}

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Display a menu in the system tray",
	Run: func(cmd *cobra.Command, _ []string) {
		args := []string{}
		for c := runCmd; c != rootCmd; c = c.Parent() {
			args = append([]string{c.Name()}, args...)
		}

		msg := strings.Join(args, " ")
		menu := &menu.Menu{}
		for _, c := range runCmd.Commands() {
			if c.Name() != "wait" {
				menu.Add(c.Name(), msg+" "+c.Name())
			}
		}

		menu.Show(cmd.Context(), &log.Logger, sockPath)
	},
}
