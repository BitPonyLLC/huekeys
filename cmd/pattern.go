package cmd

import (
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

	var delay time.Duration
	var patternCmd *cobra.Command

	addDelayFlag := func(value time.Duration) {
		patternCmd.Flags().DurationVarP(&delay, "delay", "d", value,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
	}

	patternCmd = &cobra.Command{
		Use:   "pulse",
		Short: "pulse the keyboard brightness up and down",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.BrightnessPulse(delay) },
	}
	addDelayFlag(25 * time.Millisecond)
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "rainbow",
		Short: "loop through all the colors of the rainbow",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.InfiniteRainbow(delay) },
	}
	addDelayFlag(time.Nanosecond)
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "random",
		Short: "constantly change the color to a random selection",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.InfiniteRandom(delay) },
	}
	addDelayFlag(1 * time.Second)
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "cpu",
		Short: "change the color according to CPU utilization (cold to hot)",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.MonitorCPU(delay) },
	}
	addDelayFlag(1 * time.Second)
	runCmd.AddCommand(patternCmd)

	inputEventID := ""
	patternCmd = &cobra.Command{
		Use:   "typing",
		Short: "change the color according to typing speed (cold to hot)",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.MonitorTyping(delay, inputEventID) },
	}
	addDelayFlag(300 * time.Millisecond)
	patternCmd.Flags().StringVarP(&inputEventID, "input-event-id", "i", inputEventID, "input event ID to monitor")
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "desktop",
		Short: "monitor the desktop picture and change the keyboard color to match",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.MatchDesktopBackground() },
	}
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
