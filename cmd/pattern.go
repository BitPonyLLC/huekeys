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
	runCmd.Flags().StringVarP(&Pattern, "pattern", "p", "",
		"the pattern to run: rainbow, pulse, random, cpu, desktop")
	runCmd.Flags().DurationVarP(&Delay, "delay", "d", 0,
		"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
	runCmd.Flags().IntVar(&NiceLevel, "nice", 10, "the priority level of the process")
}

const pidpath = "/tmp/sys76-kb.pid"

func checkAndSetPidPath() {
	otherPidContent, err := os.ReadFile(pidpath)
	if err == nil {
		var otherPid int
		otherPid, err = strconv.Atoi(string(otherPidContent))
		if err != nil {
			panic(err)
		}
		err = syscall.Kill(otherPid, 0)
		if err == nil {
			fmt.Fprintf(os.Stderr, "process %d is already running a pattern\n", otherPid)
			os.Exit(11)
		}
		if err.(syscall.Errno) != syscall.ESRCH {
			panic(err)
		}
	} else {
		if !os.IsNotExist(err) {
			fmt.Printf("BARF %T of %+v\n", err, err)
			panic(err)
		}
	}
	err = os.WriteFile(pidpath, []byte(fmt.Sprint(os.Getpid())), 0666)
	if err != nil {
		panic(err)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		keyboard.StopDesktopBackgroundMonitor()
		os.Remove(pidpath)
		os.Exit(0)
	}()
}

func beNice() {
	pid := syscall.Getpid()
	err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, NiceLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to set nice level %d: %v\n", NiceLevel, err)
	}
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "runs a backlight pattern",
	Long:  `runs a backlight pattern`,
	Run: func(cmd *cobra.Command, args []string) {
		if Pattern == "" {
			cmd.Help()
			return
		}
		checkAndSetPidPath()
		defer func() { os.Remove(pidpath) }()
		beNice()
		fmt.Printf("running pattern %v\n", Pattern)
		switch Pattern {
		case "rainbow":
			keyboard.InfiniteRainbow(Delay)
		case "pulse":
			keyboard.BrightnessPulse(Delay)
		case "random":
			keyboard.InfiniteRandom(Delay)
		case "cpu":
			keyboard.MonitorCPU(Delay)
		case "typing":
			keyboard.MonitorTyping(Delay, "", 0)
		case "desktop":
			keyboard.MatchDesktopBackground()
		default:
			fmt.Fprintln(os.Stderr, "unknown pattern")
			os.Exit(1)
		}
	},
}
