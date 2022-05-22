package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	keyboard "github.com/bambash/sys76-kb/pkg"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var pidpath = "/tmp/sys76-kb.pid"
var priority = 10

var cancelCtx context.Context
var cancelFunc func()

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "runs a backlight pattern",
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&pidpath, "pidpath", pidpath, "pathname of the pidfile")
	runCmd.Flags().IntVar(&priority, "nice", 10, "the priority level of the process")

	pulseDelay := 25 * time.Millisecond
	addDelayPatternCmd("pulse", "pulse the keyboard brightness up and down",
		&pulseDelay, keyboard.BrightnessPulse)

	rainbowDelay := 1 * time.Nanosecond
	addDelayPatternCmd("rainbow", "loop through all the colors of the rainbow",
		&rainbowDelay, keyboard.InfiniteRainbow)

	randomDelay := 1 * time.Second
	addDelayPatternCmd("random", "constantly change the color to a random selection",
		&randomDelay, keyboard.InfiniteRandom)

	cpuDelay := 1 * time.Second
	addDelayPatternCmd("cpu", "change the color according to CPU utilization (cold to hot)",
		&cpuDelay, keyboard.MonitorCPU)

	addPatternCmd("desktop", "monitor the desktop picture and change the keyboard color to match",
		nil, func(_ *cobra.Command, _ []string) error { return keyboard.MatchDesktopBackground(cancelCtx) })

	inputEventID := ""
	idlePattern := ""
	typingDelay := 300 * time.Millisecond
	typingPatternCmd := addPatternCmd("typing", "change the color according to typing speed (cold to hot)",
		&typingDelay,
		func(_ *cobra.Command, _ []string) error {
			var idleCB func(context.Context)
			switch idlePattern {
			case "":
			case "pulse":
				idleCB = func(ctx context.Context) { keyboard.BrightnessPulse(ctx, pulseDelay) }
			case "rainbow":
				idleCB = func(ctx context.Context) { keyboard.InfiniteRainbow(ctx, rainbowDelay) }
			case "random":
				idleCB = func(ctx context.Context) { keyboard.InfiniteRandom(ctx, randomDelay) }
			case "cpu":
				idleCB = func(ctx context.Context) { keyboard.MonitorCPU(ctx, cpuDelay) }
			case "desktop":
				idleCB = func(ctx context.Context) { keyboard.MatchDesktopBackground(ctx) }
			default:
				return fail(13, "unknown pattern: %s", idlePattern)
			}
			return keyboard.MonitorTyping(cancelCtx, typingDelay, inputEventID, idleCB)
		})

	typingPatternCmd.Flags().StringVar(&inputEventID, "input-event-id", inputEventID, "input event ID to monitor")
	typingPatternCmd.Flags().StringVarP(&idlePattern, "idle", "i", idlePattern,
		"name of pattern to run while keyboard is idle for more than 30 seconds")
}

func addDelayPatternCmd(use, short string, delay *time.Duration, patternFunc func(context.Context, time.Duration) error) {
	addPatternCmd(use, short, delay, func(_ *cobra.Command, _ []string) error {
		return patternFunc(cancelCtx, *delay)
	})
}

func addPatternCmd(use, short string, delay *time.Duration, runE func(*cobra.Command, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		PreRunE: func(_ *cobra.Command, _ []string) error {
			err := checkAndSetPidPath(pidpath)
			if err != nil {
				return fail(11, err)
			}
			err = beNice(priority)
			if err != nil {
				return fail(12, err)
			}
			return nil
		},
		RunE: runE,
		PostRun: func(_ *cobra.Command, _ []string) {
			os.Remove(pidpath)
		},
	}

	if delay != nil {
		cmd.Flags().DurationVarP(delay, "delay", "d", *delay,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
	}

	runCmd.AddCommand(cmd)
	return cmd
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

	cancelCtx, cancelFunc = context.WithCancel(context.Background())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-stop
		log.Info().Str("signal", sig.String()).Msg("stopping")
		cancelFunc()
	}()

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
