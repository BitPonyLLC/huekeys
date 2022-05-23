package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/syslog"
	"os"
	"os/signal"
	"syscall"

	"github.com/BitPonyLLC/huekeys/buildinfo"
	"github.com/BitPonyLLC/huekeys/pkg/keyboard"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var failureCode = 1
var logLevel = "info"
var logPath = ""
var logF *os.File

var rootCmd = &cobra.Command{
	Use:          buildinfo.Name,
	Short:        buildinfo.Description,
	Version:      buildinfo.All,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		err := keyboard.LoadEmbeddedColors()
		if err != nil {
			return fail(2, err)
		}
		return setupLogging(cmd)
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if logF != nil {
			logF.Close()
		}
	},
}

// Execute is the primary entrypoint for this CLI
func Execute() {
	rootCmd.SetOut(os.Stdout) // default is stderr

	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "l", logLevel, "set logging level: debug, info, warn, error")
	rootCmd.PersistentFlags().StringVar(&logPath, "log-path", logPath, "set pathname for storing logs (default: syslog)")

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-stop
		log.Info().Str("signal", sig.String()).Msg("stopping")
		cancelFunc()
	}()

	err := rootCmd.ExecuteContext(cancelCtx)
	if err != nil {
		log.Error().Err(err).Msg("command failed")
		os.Exit(failureCode)
	}

	os.Exit(0)
}

func setupLogging(cmd *cobra.Command) error {
	var logWriter io.Writer

	if logPath == "" {
		syslogger, err := syslog.New(syslog.LOG_INFO, buildinfo.Name)
		if err != nil {
			return fail(3, err)
		}

		logWriter = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.NoColor = true
			w.PartsExclude = []string{zerolog.TimestampFieldName}
			w.Out = zerolog.SyslogLevelWriter(syslogger)
		})
	} else {
		logF, err := os.Open(logPath)
		if err != nil {
			return fail(4, "unable to open %s: %w", logPath, err)
		}

		logWriter = logF
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		return fail(4, err)
	}

	zerolog.SetGlobalLevel(level)
	log.Logger = zerolog.New(logWriter)
	return nil
}

func fail(code int, formatOrErr interface{}, args ...interface{}) error {
	failureCode = code
	if len(args) == 0 {
		err, ok := formatOrErr.(error)
		if ok {
			return err
		}
		return errors.New(formatOrErr.(string))
	}
	return fmt.Errorf(formatOrErr.(string), args...)
}
