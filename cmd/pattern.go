package cmd

import (
	"fmt"
	"os"
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
	runCmd.Flags().StringVarP(&Pattern, "pattern", "p", "", "the pattern to run: rainbow, pulse, random")
	runCmd.Flags().DurationVarP(&Delay, "delay", "d", 0,
		"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
	runCmd.Flags().IntVar(&NiceLevel, "nice", 10, "the priority level of the process")
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
	Long:  `runs a pattern that the backlight loops through. 'rainbow' or 'pulse'`,
	Run: func(cmd *cobra.Command, args []string) {
		if Pattern != "" {
			beNice()
			fmt.Printf("running pattern %v\n", Pattern)
			switch Pattern {
			case "rainbow":
				keyboard.InfiniteRainbow(Delay)
			case "pulse":
				keyboard.BrightnessPulse(Delay)
			case "random":
				keyboard.InfiniteRandom(Delay)
			default:
				fmt.Fprintln(os.Stderr, "unknown pattern")
				os.Exit(1)
			}
		} else {
			cmd.Help()
		}
	},
}
