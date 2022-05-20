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
	"github.com/spf13/cobra"
)

func init() {
	pidpath := "/tmp/sys76-kb.pid"
	priority := 10

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "runs a backlight pattern",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			err := checkAndSetPidPath(pidpath)
			if err != nil {
				cmd.PrintErr(err)
				os.Exit(1)
			}

			err = beNice(priority)
			if err != nil {
				cmd.PrintErr(err)
				os.Exit(2)
			}
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			os.Remove(pidpath)
		},
	}
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&pidpath, "pidpath", pidpath, "pathname of the pidfile")
	runCmd.Flags().IntVar(&priority, "nice", 10, "the priority level of the process")

	var patternCmd *cobra.Command

	addDelayFlag := func(delay *time.Duration) {
		patternCmd.Flags().DurationVarP(delay, "delay", "d", *delay,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
	}

	pulseDelay := 25 * time.Millisecond
	patternCmd = &cobra.Command{
		Use:   "pulse",
		Short: "pulse the keyboard brightness up and down",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.BrightnessPulse(context.Background(), pulseDelay) },
	}
	addDelayFlag(&pulseDelay)
	runCmd.AddCommand(patternCmd)

	rainbowDelay := time.Nanosecond
	patternCmd = &cobra.Command{
		Use:   "rainbow",
		Short: "loop through all the colors of the rainbow",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.InfiniteRainbow(context.Background(), rainbowDelay) },
	}
	addDelayFlag(&rainbowDelay)
	runCmd.AddCommand(patternCmd)

	randomDelay := 1 * time.Second
	patternCmd = &cobra.Command{
		Use:   "random",
		Short: "constantly change the color to a random selection",
		Run: func(_ *cobra.Command, _ []string) {
			keyboard.InfiniteRandom(context.Background(), randomDelay)
		},
	}
	addDelayFlag(&randomDelay)
	runCmd.AddCommand(patternCmd)

	cpuDelay := 1 * time.Second
	patternCmd = &cobra.Command{
		Use:   "cpu",
		Short: "change the color according to CPU utilization (cold to hot)",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.MonitorCPU(context.Background(), cpuDelay) },
	}
	addDelayFlag(&cpuDelay)
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "desktop",
		Short: "monitor the desktop picture and change the keyboard color to match",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.MatchDesktopBackground(context.Background()) },
	}
	runCmd.AddCommand(patternCmd)

	typingDelay := 300 * time.Millisecond
	inputEventID := ""
	idlePattern := ""
	patternCmd = &cobra.Command{
		Use:   "typing",
		Short: "change the color according to typing speed (cold to hot)",
		Run: func(cmd *cobra.Command, _ []string) {
			var idleCB func(context.Context)
			switch idlePattern {
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
				cmd.PrintErrln("unknown pattern:", idlePattern)
				os.Exit(3)
			}
			keyboard.MonitorTyping(context.Background(), typingDelay, inputEventID, idleCB)
		},
	}
	addDelayFlag(&typingDelay)
	patternCmd.Flags().StringVar(&inputEventID, "input-event-id", inputEventID, "input event ID to monitor")
	patternCmd.Flags().StringVarP(&idlePattern, "idle", "i", idlePattern,
		"name of pattern to run while keyboard is idle for more than 30 seconds")
	runCmd.AddCommand(patternCmd)
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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		keyboard.StopDesktopBackgroundMonitor()
		os.Remove(pidpath)
		os.Exit(0)
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
