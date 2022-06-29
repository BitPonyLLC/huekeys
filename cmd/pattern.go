package cmd

import (
	"os"
	"path/filepath"

	"github.com/BitPonyLLC/huekeys/buildinfo"
	"github.com/BitPonyLLC/huekeys/pkg/patterns"
	"github.com/BitPonyLLC/huekeys/pkg/pidpath"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var waitPidPath *pidpath.PidPath

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "runs a backlight pattern",
}

func init() {
	patterns.SetConfig(viper.GetViper())

	rootCmd.AddCommand(runCmd)

	//----------------------------------------
	addPatternCmd("pulse the keyboard brightness up and down", patterns.Get("pulse"))
	addPatternCmd("loop through all the colors of the rainbow", patterns.Get("rainbow"))
	addPatternCmd("constantly change the color to a random selection", patterns.Get("random"))
	addPatternCmd("change the color according to CPU utilization (cold to hot)", patterns.Get("cpu"))
	addPatternCmd("monitor the desktop picture and change the keyboard color to match", patterns.Get("desktop"))

	//----------------------------------------
	desktopEnv := ""
	waitCmd := addPatternCmd("wait for remote commands", patterns.Get("wait"))
	// wait needs to manage the pidpath and start the IPC server...
	waitCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if err := commonPreRunE(cmd, args); err != nil {
			return err
		}
		if err := waitPidPath.CheckAndSet(); err != nil {
			return fail(11, err)
		}
		if desktopEnv != "" {
			desktopPattern := patterns.Get("desktop").(*patterns.DesktopPattern)
			if err := desktopPattern.SetEnv(desktopEnv); err != nil {
				return err
			}
		}
		return ipcServer.Start(cmd.Context(), &log.Logger, waitSockPath(), rootCmd)
	}
	waitCmd.PostRun = func(cmd *cobra.Command, args []string) {
		if waitPidPath != nil {
			waitPidPath.Release()
		}
	}

	defaultPidPath := filepath.Join(os.TempDir(), buildinfo.App.Name+"-wait.pid")
	waitCmd.Flags().String("pidpath", defaultPidPath, "pathname of the wait pidfile")
	viper.BindPFlag("wait.pidpath", waitCmd.Flags().Lookup("pidpath"))

	defaultSockPath := filepath.Join(os.TempDir(), buildinfo.App.Name+"-wait.sock")
	waitCmd.Flags().String("sockpath", defaultSockPath, "pathname of the wait sockfile")
	viper.BindPFlag("wait.sockpath", waitCmd.Flags().Lookup("sockpath"))

	waitCmd.Flags().Duration(patterns.MonitorLabel, 0, "monitor and preserve set color and/or brightness")
	viper.BindPFlag("wait."+patterns.MonitorLabel, waitCmd.Flags().Lookup(patterns.MonitorLabel))

	waitCmd.Flags().StringVar(&desktopEnv, "env", desktopEnv, "environment to set for desktop pattern")
	waitCmd.Flags().MarkHidden("env") // only used by menu

	//----------------------------------------
	watchPattern := patterns.Get("watch").(*patterns.WatchPattern)
	watchCmd := addPatternCmd("watch and report color, brightness, and pattern changes", watchPattern)
	// watch needs to behave differently from others when run...
	watchCmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if waitPidPath.IsRunning() && !waitPidPath.IsOurs() {
			return sendViaIPCForeground(cmd, true, "")
		}
		// there may be multiple watch patterns running (i.e. multiple watch
		// clients) so each one needs to maintain its own Out writer!
		pattern := &patterns.WatchPattern{}
		*pattern = *watchPattern
		pattern.Out = cmd.OutOrStdout()
		return pattern.Run(cmd.Context(), &log.Logger)
	}

	//----------------------------------------
	typingPattern := patterns.Get("typing")
	typingLabel := typingPattern.GetBase().Name + "."

	typeCmd := addPatternCmd("change the color according to typing speed (cold to hot)", typingPattern)

	typeCmd.Flags().String(patterns.InputEventIDLabel, "", "input event ID to monitor")
	viper.BindPFlag(typingLabel+patterns.InputEventIDLabel, typeCmd.Flags().Lookup(patterns.InputEventIDLabel))

	typeCmd.Flags().Bool(patterns.AllKeysLabel, false, "count any key pressed instead of only those that are considered \"printable\"")
	viper.BindPFlag(typingLabel+patterns.AllKeysLabel, typeCmd.Flags().Lookup(patterns.AllKeysLabel))

	typeCmd.Flags().StringP(patterns.IdleLabel, "i", "", "name of pattern to run while keyboard is idle for more than the idle period")
	viper.BindPFlag(typingLabel+patterns.IdleLabel, typeCmd.Flags().Lookup(patterns.IdleLabel))

	typeCmd.Flags().DurationP(patterns.IdlePeriodLabel, "p", patterns.DefaultIdlePeriod, "amount of idle time to wait before starting the idle pattern")
	viper.BindPFlag(typingLabel+patterns.IdlePeriodLabel, typeCmd.Flags().Lookup(patterns.IdlePeriodLabel))
}

func addPatternCmd(short string, pattern patterns.Pattern) *cobra.Command {
	basePattern := pattern.GetBase()

	cmd := &cobra.Command{
		Use:     pattern.GetBase().Name,
		Short:   short,
		PreRunE: commonPreRunE,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if waitPidPath.IsRunning() && !waitPidPath.IsOurs() {
				return sendViaIPC(cmd)
			}
			return pattern.Run(cmd.Context(), &log.Logger)
		},
	}

	defaultDelay := pattern.GetDefaultDelay()
	if defaultDelay != 0 {
		cmd.Flags().DurationP("delay", "d", defaultDelay,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
		viper.BindPFlag(basePattern.Name+".delay", cmd.Flags().Lookup("delay"))
	}

	runCmd.AddCommand(cmd)
	return cmd
}

func commonPreRunE(cmd *cobra.Command, _ []string) error {
	return util.BeNice(viper.GetInt("nice"))
}

func waitSockPath() string {
	return viper.GetString("wait.sockpath")
}
