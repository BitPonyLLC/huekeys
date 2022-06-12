package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/BitPonyLLC/huekeys/buildinfo"
	"github.com/BitPonyLLC/huekeys/internal/menu"
	"github.com/BitPonyLLC/huekeys/pkg/patterns"
	"github.com/BitPonyLLC/huekeys/pkg/pidpath"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var patternName string
var menuPidPath *pidpath.PidPath
var restarting = false

func init() {
	menuCmd.Flags().StringVarP(&patternName, "pattern", "p", patternName, "name of pattern to run at start")
	viper.BindPFlag("menu.pattern", menuCmd.Flags().Lookup("pattern"))

	defaultPidPath := filepath.Join(os.TempDir(), buildinfo.App.Name+"-menu.pid")
	menuCmd.Flags().String("pidpath", defaultPidPath, "pathname of the menu pidfile")
	viper.BindPFlag("menu.pidpath", menuCmd.Flags().Lookup("pidpath"))

	rootCmd.AddCommand(menuCmd)
}

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Display a menu in the system tray",
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		err := menuPidPath.CheckAndSet()
		if err != nil {
			return err
		}

		return ensureWaitRunning(cmd)
	},
	RunE: func(cmd *cobra.Command, _ []string) error {
		restart := make(chan os.Signal, 1)
		signal.Notify(restart, syscall.SIGHUP)
		go func() {
			sig := <-restart
			restarting = true
			log.Info().Str("signal", sig.String()).Msg("restarting")
			cancelFunc()
		}()

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

		return menu.Show(cmd.Context(), &log.Logger, waitSockPath())
	},
	PostRun: func(_ *cobra.Command, _ []string) {
		if menuPidPath != nil {
			menuPidPath.Release()
		}

		if !restarting {
			return
		}

		mCmd := exec.Command(os.Args[0], os.Args[1:]...)
		err := mCmd.Start()
		if err != nil {
			log.Error().Err(err).Msg("failed to restart menu")
		}

		log.Info().Str("cmd", mCmd.String()).Interface("menu", mCmd.Process).Msg("new menu process started")
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

func ensureWaitRunning(cmd *cobra.Command) error {
	if !waitPidPath.IsOurs() && waitPidPath.IsRunning() {
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

	config := viper.GetViper().ConfigFileUsed()
	if config != "" {
		config = " --config " + config
	}

	// use sh exec to let parent processes exit
	hkCmd := fmt.Sprint("export ", dpEnv, "; exec ", exe, config, " run wait &")
	execArgs = append(execArgs, "sh", "-c", hkCmd)

	execStr := fmt.Sprint(execName, " ", strings.Join(execArgs, " "))
	log.Debug().Str("cmd", execStr).Msg("")

	err = exec.Command(execName, execArgs...).Run()
	if err != nil {
		return fmt.Errorf("unable to run %s: %w", execStr, err)
	}

	// wait a second for socket to be ready...
	sockPath := waitSockPath()
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
