package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/BitPonyLLC/huekeys/internal/menu"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var patternName string

func init() {
	menuCmd.Flags().StringVarP(&patternName, "pattern", "p", patternName, "name of pattern to run at start")
	viper.BindPFlag("menu.pattern", menuCmd.Flags().Lookup("pattern"))
	rootCmd.AddCommand(menuCmd)
}

var menuCmd = &cobra.Command{
	Use:     "menu",
	Short:   "Display a menu in the system tray",
	PreRunE: ensureWaitRunning,
	RunE: func(cmd *cobra.Command, _ []string) error {
		args := []string{}
		for c := runCmd; c != rootCmd; c = c.Parent() {
			args = append([]string{c.Name()}, args...)
		}

		msg := strings.Join(args, " ")
		menu := &menu.Menu{PatternName: viper.GetString("menu.pattern")}
		for _, c := range runCmd.Commands() {
			if c.Name() != "wait" {
				menu.Add(c.Name(), msg+" "+c.Name())
			}
		}

		return menu.Show(cmd.Context(), &log.Logger, viper.GetString("sockpath"))
	},
	Args: func(cmd *cobra.Command, args []string) error {
		if patternName == "" {
			return nil
		}

		for _, cmd := range runCmd.Commands() {
			if cmd.Name() == patternName {
				return nil
			}
		}

		return fmt.Errorf("unknown pattern: %s", patternName)
	},
}

func ensureWaitRunning(cmd *cobra.Command, args []string) error {
	if !pidPath.IsOurs() && pidPath.IsRunning() {
		// wait is already executing in the background
		return nil
	}

	// use sh exec to remove sudo parent processes hanging around
	hkCmd := "exec " + os.Args[0] + " run wait &"
	execArgs := []string{"sudo", "-E", "sh", "-c", hkCmd}
	execStr := strings.Join(execArgs, " ")
	log.Debug().Str("cmd", execStr).Msg("")

	err := exec.Command("sudo", "-E", "sh", "-c", hkCmd).Run()
	if err != nil {
		return fmt.Errorf("unable to run %s: %w", execStr, err)
	}

	// wait a second for socket to be ready...
	sockPath := viper.GetString("sockpath")
	for i := 0; i < 10; i += 1 {
		time.Sleep(50 * time.Millisecond)
		_, err := os.Stat(sockPath)
		if err == nil {
			return nil
		}

		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to stat %s: %w", sockPath, err)
		}
	}

	return fmt.Errorf("unable to start background process as root")
}
