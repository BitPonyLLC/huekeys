package cmd

import (
	"fmt"
	"io"
	"log/syslog"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var logLevel = "info"
var logPath = ""
var logF *os.File

var rootCmd = &cobra.Command{
	Use:   "sys76-kb",
	Short: "sys76-kb is a keyboard controller for System76 laptops",
	Long: `A simple keyboard contoller built with
		   love by bambash in Go.
		   Complete documentation is available at https://github.com/bambash/sys76-kb`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) { setupLogging(cmd) },
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		if err := cmd.ParseFlags(args); err != nil {
			fmt.Printf("Error parsing flags: %v\n", err)
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if logF != nil {
			logF.Close()
		}
	},
}

// Execute is the primary entrypoint for this CLI
func Execute() {
	rootCmd.Flags().StringVarP(&logLevel, "log-level", "l", logLevel, "set logging level: debug, info, warn, error")
	rootCmd.Flags().StringVar(&logPath, "log-path", logPath, "set pathname for storing logs (default: syslog)")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setupLogging(cmd *cobra.Command) {
	var logWriter io.Writer

	if logPath == "" {
		syslogger, err := syslog.New(syslog.LOG_INFO, "sys76-kb")
		if err != nil {
			cmd.PrintErrln(err)
			os.Exit(2)
		}

		logWriter = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.NoColor = true
			w.PartsExclude = []string{zerolog.TimestampFieldName}
			w.Out = zerolog.SyslogLevelWriter(syslogger)
		})
	} else {
		logF, err := os.Open(logPath)
		if err != nil {
			cmd.PrintErrf("unable to open %s: %v\n", logPath, err)
			os.Exit(3)
		}

		logWriter = logF
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		cmd.PrintErrln(err)
		os.Exit(4)
	}

	zerolog.SetGlobalLevel(level)
	log.Logger = zerolog.New(logWriter)
}
