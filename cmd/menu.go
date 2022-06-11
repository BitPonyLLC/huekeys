package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/BitPonyLLC/huekeys/buildinfo"
	"github.com/BitPonyLLC/huekeys/internal/menu"
	"github.com/BitPonyLLC/huekeys/pkg/patterns"
	"github.com/BitPonyLLC/huekeys/pkg/util"

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

		menu := &menu.Menu{
			PatternName: viper.GetString("menu.pattern"),
			AboutInfo:   buildinfo.App.Name + " " + buildinfo.App.Version,
		}

		msg := strings.Join(args, " ")
		for _, c := range runCmd.Commands() {
			if c.Name() != "wait" && c.Name() != "watch" {
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

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to determine executable pathname: %w", err)
	}

	dpEnv, err := patterns.DesktopPatternEnv()
	if err != nil {
		if util.IsTTY(os.Stderr) {
			cmd.PrintErrln(err)
		} else {
			log.Warn().Err(err).Msg("")
		}
	}

	var execName string
	var execArgs []string

	// checking only stdin isn't enough: it's attached when launched from gnome!
	if util.IsTTY(os.Stdin) && util.IsTTY(os.Stdout) {
		execName = "sudo"
		execArgs = []string{}
	} else {
		// need to open a dialog for permission...
		execName = "pkexec"
		execArgs = []string{"--user", "root"}
	}

	// use sh exec to let parent processes exit
	hkCmd := fmt.Sprint("export ", dpEnv, "; exec ", exe,
		" --config ", viper.GetViper().ConfigFileUsed(), " run wait &")
	execArgs = append(execArgs, "sh", "-c", hkCmd)

	execStr := fmt.Sprint(execName, " ", strings.Join(execArgs, " "))
	log.Debug().Str("cmd", execStr).Msg("")

	err = exec.Command(execName, execArgs...).Run()
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
