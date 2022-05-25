package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/BitPonyLLC/huekeys/buildinfo"
	"github.com/BitPonyLLC/huekeys/pkg/patterns"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "runs a backlight pattern",
}

func init() {
	rootCmd.AddCommand(runCmd)

	defaultPidPath := filepath.Join(os.TempDir(), buildinfo.Name+".pid")
	runCmd.PersistentFlags().String("pidpath", defaultPidPath, "pathname of the pidfile")
	viper.BindPFlag("pidpath", runCmd.PersistentFlags().Lookup("pidpath"))

	runCmd.PersistentFlags().Int("nice", 10, "the priority level of the process")
	viper.BindPFlag("nice", runCmd.PersistentFlags().Lookup("nice"))

	addPatternCmd("pulse the keyboard brightness up and down", patterns.NewPulsePattern())
	addPatternCmd("loop through all the colors of the rainbow", patterns.NewRainbowPattern())
	addPatternCmd("constantly change the color to a random selection", patterns.NewRandomPattern())
	addPatternCmd("change the color according to CPU utilization (cold to hot)", patterns.NewCPUPattern())
	addPatternCmd("monitor the desktop picture and change the keyboard color to match", patterns.NewDesktopPattern())

	typingPattern := patterns.NewTypingPattern()
	typingPatternCmd := addPatternCmd("change the color according to typing speed (cold to hot)", typingPattern)

	typingPatternCmd.Flags().String("input-event-id", typingPattern.InputEventID, "input event ID to monitor")
	viper.BindPFlag("typing.input-event-id", typingPatternCmd.Flags().Lookup("input-event-id"))

	typingPatternCmd.Flags().StringP("idle", "i", "", "name of pattern to run while keyboard is idle for more than the idle period")
	viper.BindPFlag("typing.idle", typingPatternCmd.Flags().Lookup("idle"))

	typingPatternCmd.Flags().DurationP("idle-period", "p", typingPattern.IdlePeriod, "amount of idle time to wait before starting the idle pattern")
	viper.BindPFlag("typing.idle-period", typingPatternCmd.Flags().Lookup("idle-period"))

	typingPatternCmd.Args = func(cmd *cobra.Command, _ []string) (err error) {
		typingPattern.InputEventID = viper.GetString("typing.input-event-id")
		typingPattern.IdlePattern, err = getIdlePattern(cmd, viper.GetString("typing.idle"))
		return
	}
}

func addPatternCmd(short string, pattern patterns.Pattern) *cobra.Command {
	pidpath := viper.GetString("pidpath")
	priority := viper.GetInt("nice")
	basePattern := pattern.GetBase()
	cmd := &cobra.Command{
		Use:   pattern.GetBase().Name,
		Short: short,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			err := checkAndSetPidPath(pidpath)
			if err != nil {
				return fail(11, err)
			}

			err = beNice(priority)
			if err != nil {
				return fail(12, err)
			}

			plog := log.With().Str("cmd", cmd.Name()).Logger()
			plog.Info().Msg("starting")

			basePattern.Ctx = cmd.Context()
			basePattern.Log = &plog

			return startIPCServer(cmd.Context())
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			basePattern.Delay = viper.GetDuration(basePattern.Name + ".delay")
			println("BARF waiting, doing nothing...")
			<-cmd.Context().Done()
			return nil
			// return pattern.Run()
		},
		PostRun: func(_ *cobra.Command, _ []string) {
			os.Remove(pidpath)
		},
	}

	if basePattern.Delay != 0 {
		cmd.Flags().DurationP("delay", "d", basePattern.Delay,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
		viper.BindPFlag(basePattern.Name+".delay", cmd.Flags().Lookup("delay"))
	}

	runCmd.AddCommand(cmd)
	return cmd
}

func getIdlePattern(cmd *cobra.Command, patternName string) (patterns.Pattern, error) {
	switch patternName {
	case "":
		return nil, nil
	case "pulse":
		return patterns.NewPulsePattern(), nil
	case "rainbow":
		return patterns.NewRainbowPattern(), nil
	case "random":
		return patterns.NewRandomPattern(), nil
	case "cpu":
		return patterns.NewCPUPattern(), nil
	case "desktop":
		return patterns.NewDesktopPattern(), nil
	default:
		return nil, fail(13, "unknown pattern: %s", patternName)
	}
}

func checkAndSetPidPath(pidpath string) error {
	otherPidContent, err := os.ReadFile(pidpath)
	if err == nil {
		var otherPid int
		otherPid, err = strconv.Atoi(string(otherPidContent))
		if err != nil {
			return fmt.Errorf("unable to parse contents of %s: %w", pidpath, err)
		}

		err = syscall.Kill(otherPid, 0)
		if err == nil || err.(syscall.Errno) == syscall.EPERM {
			// if EPERM, process is owned by another user, probably root
			return fmt.Errorf("process %d is already running a pattern", otherPid)
		}

		// ESRCH: no such process
		if err.(syscall.Errno) != syscall.ESRCH {
			return fmt.Errorf("unable to check if process %d is still running: %w", otherPid, err)
		}
	} else {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable to read %s: %w", pidpath, err)
		}
	}

	err = os.WriteFile(pidpath, []byte(fmt.Sprint(os.Getpid())), 0666)
	if err != nil {
		return fmt.Errorf("unable to write to %s: %w", pidpath, err)
	}

	return nil
}

func beNice(priority int) error {
	pid := syscall.Getpid()
	err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, priority)
	if err != nil {
		return fmt.Errorf("unable to set nice level %d: %w", priority, err)
	}
	return nil
}
