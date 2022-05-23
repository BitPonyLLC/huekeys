package cmd

import (
	"fmt"
	"os"
	"strconv"
	"syscall"

	"github.com/BitPonyLLC/huekeys/buildinfo"
	"github.com/BitPonyLLC/huekeys/pkg/patterns"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var pidpath = "/tmp/" + buildinfo.Name + ".pid"
var priority = 10

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "runs a backlight pattern",
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.PersistentFlags().StringVar(&pidpath, "pidpath", pidpath, "pathname of the pidfile")
	runCmd.PersistentFlags().IntVar(&priority, "nice", 10, "the priority level of the process")

	addPatternCmd("pulse", "pulse the keyboard brightness up and down", patterns.NewPulsePattern())
	addPatternCmd("rainbow", "loop through all the colors of the rainbow", patterns.NewRainbowPattern())
	addPatternCmd("random", "constantly change the color to a random selection", patterns.NewRandomPattern())
	addPatternCmd("cpu", "change the color according to CPU utilization (cold to hot)", patterns.NewCPUPattern())
	addPatternCmd("desktop", "monitor the desktop picture and change the keyboard color to match", patterns.NewDesktopPattern())

	typingPattern := patterns.NewTypingPattern()
	typingPatternCmd := addPatternCmd("typing", "change the color according to typing speed (cold to hot)", typingPattern)
	typingPatternCmd.Flags().StringVar(&typingPattern.InputEventID, "input-event-id", typingPattern.InputEventID,
		"input event ID to monitor")

	idlePatternName := ""
	typingPatternCmd.Flags().StringVarP(&idlePatternName, "idle", "i", idlePatternName,
		"name of pattern to run while keyboard is idle for more than 30 seconds")
	typingPatternCmd.Args = func(cmd *cobra.Command, _ []string) (err error) {
		typingPattern.IdlePattern, err = getIdlePattern(cmd, idlePatternName)
		return
	}
}

func addPatternCmd(use, short string, pattern patterns.Pattern) *cobra.Command {
	basePattern := pattern.GetBase()
	cmd := &cobra.Command{
		Use:   use,
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

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return pattern.Run()
		},
		PostRun: func(_ *cobra.Command, _ []string) {
			os.Remove(pidpath)
		},
	}

	if basePattern.Delay != 0 {
		cmd.Flags().DurationVarP(&basePattern.Delay, "delay", "d", basePattern.Delay,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
	}

	runCmd.AddCommand(cmd)
	return cmd
}

func getIdlePattern(cmd *cobra.Command, patternName string) (patterns.Pattern, error) {
	switch patternName {
	case "":
		// no pattern specified, nothing to do
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
