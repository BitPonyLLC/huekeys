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

	menuCmd.Flags().Duration("delay", 0, "delay before asking for sudo permission")
	viper.BindPFlag("menu.delay", menuCmd.Flags().Lookup("delay"))

	viper.SetDefault("menu.autostart", false)

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

	delay := viper.GetDuration("menu.delay")
	if delay > time.Second {
		log.Debug().Dur("delay", delay).Msg("waiting")
	}

	// this is useful when launching at start of X session to let the
	// windowing env get set up
	time.Sleep(delay)

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

	execArgs = append(execArgs, buildinfo.App.ExePath)

	config := viper.GetViper().ConfigFileUsed()
	if config != "" {
		execArgs = append(execArgs, "--config", config)
	}

	execArgs = append(execArgs, "run", "wait", "--env", dpEnv)

	execStr := fmt.Sprint(execName, " ", strings.Join(execArgs, " "))
	log.Debug().Str("cmd", execStr).Msg("")

	subCmd := exec.Command(execName, execArgs...)

	// send out/err to logs...
	subLogger := log.With().Str("cmd", execName).Logger()
	subCmd.Stdout = &util.CommandLogger{Log: func(msg string) { subLogger.Info().Msg(msg) }}
	subCmd.Stderr = &util.CommandLogger{Log: func(msg string) { subLogger.Error().Msg(msg) }}

	err = subCmd.Start()
	if err != nil {
		return fmt.Errorf("unable to run %s: %w", execStr, err)
	}

	var subCmdErr error

	go func() {
		stat, err := subCmd.Process.Wait()
		if err == nil {
			subCmdErr = fmt.Errorf("wait process exited: %s", stat)
		} else {
			subCmdErr = fmt.Errorf("unable to stat the background wait process: %w", err)
		}
	}()

	// wait for user to grant permission to run...
	const wait = 50 * time.Millisecond
	sockPath := waitSockPath()
	for timeout := time.Minute; timeout > 0; timeout -= wait {
		if subCmdErr != nil {
			return subCmdErr
		}

		time.Sleep(wait)
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
