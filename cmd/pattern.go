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

// Pattern represents keyboard color pattern to run
var Pattern string

// Delay represents the amount of time to wait between updates
var Delay time.Duration

// NiceLevel represents the priority of the process
var NiceLevel int

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().IntVar(&NiceLevel, "nice", 10, "the priority level of the process")

	var patternCmd *cobra.Command

	addDelayFlag := func(value time.Duration) {
		patternCmd.Flags().DurationVarP(&Delay, "delay", "d", value,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
	}

	patternCmd = &cobra.Command{
		Use:   "pulse",
		Short: "pulse the keyboard brightness up and down",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.BrightnessPulse(Delay) },
	}
	addDelayFlag(25 * time.Millisecond)
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "rainbow",
		Short: "loop through all the colors of the rainbow",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.InfiniteRainbow(Delay) },
	}
	addDelayFlag(time.Nanosecond)
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "random",
		Short: "constantly change the color to a random selection",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.InfiniteRandom(Delay) },
	}
	addDelayFlag(1 * time.Second)
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "cpu",
		Short: "change the color according to CPU utilization (cold to hot)",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.MonitorCPU(Delay) },
	}
	addDelayFlag(1 * time.Second)
	runCmd.AddCommand(patternCmd)

	inputEventID := ""
	patternCmd = &cobra.Command{
		Use:   "typing",
		Short: "change the color according to typing speed (cold to hot)",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.MonitorTyping(Delay, inputEventID) },
	}
	addDelayFlag(300 * time.Millisecond)
	patternCmd.Flags().StringVarP(&inputEventID, "input-event-id", "i", inputEventID,
		"input event ID to monitor")
	runCmd.AddCommand(patternCmd)

	patternCmd = &cobra.Command{
		Use:   "desktop",
		Short: "monitor the desktop picture and change the keyboard color to match",
		Run:   func(_ *cobra.Command, _ []string) { keyboard.MatchDesktopBackground() },
	}
	runCmd.AddCommand(patternCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "runs a backlight pattern",
	Long:  `runs a backlight pattern`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := checkAndSetPidPath()
		if err != nil {
			return err
		}

		err = beNice()
		if err != nil {
			return err
		}

		fmt.Printf("running pattern %v\n", Pattern)
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		os.Remove(pidpath)
	},
}

const pidpath = "/tmp/sys76-kb.pid"

func checkAndSetPidPath() error {
	otherPidContent, err := os.ReadFile(pidpath)
	if err == nil {
		var otherPid int
		otherPid, err = strconv.Atoi(string(otherPidContent))
		if err != nil {
			panic(err)
		}

		err = syscall.Kill(otherPid, 0)
		if err == nil {
			return fmt.Errorf("process %d is already running a pattern", otherPid)
		}

		if err.(syscall.Errno) != syscall.ESRCH {
			return err
		}
	} else {
		if !os.IsNotExist(err) {
			return err
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

func beNice() error {
	pid := syscall.Getpid()
	err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, NiceLevel)
	if err != nil {
		return fmt.Errorf("unable to set nice level %d: %w", NiceLevel, err)
	}
	return nil
}
