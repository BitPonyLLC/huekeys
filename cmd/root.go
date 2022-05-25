package cmd

import (
	"bufio"
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
	"github.com/rs/zerolog/pkgerrors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var failureCode = 1
var dumpConfig = false
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
	RunE: func(cmd *cobra.Command, _ []string) error {
		if dumpConfig {
			return showConfig()
		}
		return cmd.Help()
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		if logF != nil {
			logF.Close()
		}
	},
}

// Execute is the primary entrypoint for this CLI
func Execute() {
	viper.SetConfigName("." + buildinfo.Name)
	viper.SetConfigType("toml")
	viper.AddConfigPath("$HOME")

	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			rootCmd.PrintErrln("Unable to read config file:", err)
			os.Exit(1)
		}
	}

	rootCmd.SetOut(os.Stdout) // default is stderr

	rootCmd.PersistentFlags().BoolVar(&dumpConfig, "dump-config", dumpConfig, "dump configuration to stdout")

	rootCmd.PersistentFlags().String("log-level", "info", "set logging level: debug, info, warn, error")
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))

	rootCmd.PersistentFlags().String("log-dst", "syslog", "write logs to syslog, stdout, stderr, or provide a pathname")
	viper.BindPFlag("log-dst", rootCmd.PersistentFlags().Lookup("log-dst"))

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-stop
		log.Info().Str("signal", sig.String()).Msg("stopping")
		cancelFunc()
	}()

	err = rootCmd.ExecuteContext(cancelCtx)
	if err != nil {
		log.Error().Err(err).Msg("command failed")
		os.Exit(failureCode)
	}

	os.Exit(0)
}

func showConfig() error {
	tf, err := os.CreateTemp(os.TempDir(), buildinfo.Name)
	if err != nil {
		return err
	}

	defer func() {
		tf.Close()
		os.Remove(tf.Name())
	}()

	err = viper.WriteConfigAs(tf.Name())
	if err != nil {
		return err
	}

	_, err = tf.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(tf)
	for scanner.Scan() {
		fmt.Fprintln(os.Stdout, scanner.Text())
	}

	return nil
}

const minimalTimeFormat = "15:04:05.000"

func setupLogging(cmd *cobra.Command) error {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	var logWriter io.Writer

	withTime := true
	logDst := viper.GetString("log-dst")

	switch logDst {
	case "syslog":
		syslogger, err := syslog.New(syslog.LOG_INFO, buildinfo.Name)
		if err != nil {
			return fail(3, err)
		}

		withTime = false
		logWriter = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.NoColor = true
			w.PartsExclude = []string{zerolog.TimestampFieldName}
			w.Out = zerolog.SyslogLevelWriter(syslogger)
		})
	case "stdout":
		zerolog.TimeFieldFormat = minimalTimeFormat
		logWriter = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.TimeFormat = minimalTimeFormat
			w.Out = os.Stdout
		})
	case "stderr":
		zerolog.TimeFieldFormat = minimalTimeFormat
		logWriter = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.TimeFormat = minimalTimeFormat
			w.Out = os.Stderr
		})
	default:
		logF, err := os.OpenFile(logDst, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return fail(4, "unable to open %s: %w", logDst, err)
		}

		logWriter = logF
	}

	level, err := zerolog.ParseLevel(viper.GetString("log-level"))
	if err != nil {
		return fail(4, err)
	}

	zerolog.SetGlobalLevel(level)

	if withTime {
		log.Logger = zerolog.New(logWriter).With().Timestamp().Logger()
	} else {
		log.Logger = zerolog.New(logWriter)
	}

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
