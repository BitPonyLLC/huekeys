package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/syslog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/BitPonyLLC/huekeys/buildinfo"
	"github.com/BitPonyLLC/huekeys/pkg/ipc"
	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
	"github.com/BitPonyLLC/huekeys/pkg/pidpath"
	"github.com/BitPonyLLC/huekeys/pkg/termwrap"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Execute is the primary entrypoint for this CLI
func Execute() int {
	defer atExit()

	tw := termwrap.NewTermWrap(80, 24)
	rootCmd.Long = tw.Paragraph(buildinfo.App.Description + "\n\n" + buildinfo.App.FullDescription)

	rootCmd.SetOut(os.Stdout) // default is stderr

	err := keyboard.LoadEmbeddedColors()
	if err != nil {
		rootCmd.PrintErrln("Unable to load colors:", err)
		return 2
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", configPath, "the configuration file to load")
	rootCmd.Flags().BoolVar(&dumpConfig, "dump-config", dumpConfig, "dump configuration to stdout")

	rootCmd.PersistentFlags().String("log-level", "info", "set logging level: debug, info, warn, error")
	viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level"))

	rootCmd.PersistentFlags().String(logDstLabel, "syslog", "write logs to syslog, stdout, stderr, or provide a pathname")
	viper.BindPFlag(logDstLabel, rootCmd.PersistentFlags().Lookup(logDstLabel))

	defaultPidPath := filepath.Join(os.TempDir(), buildinfo.App.Name+".pid")
	rootCmd.PersistentFlags().String("pidpath", defaultPidPath, "pathname of the pidfile")
	viper.BindPFlag("pidpath", rootCmd.PersistentFlags().Lookup("pidpath"))

	defaultSockPath := filepath.Join(os.TempDir(), buildinfo.App.Name+".sock")
	rootCmd.PersistentFlags().String("sockpath", defaultSockPath, "pathname of the sockfile")
	viper.BindPFlag("sockpath", rootCmd.PersistentFlags().Lookup("sockpath"))

	rootCmd.PersistentFlags().Int("nice", 10, "the priority level of the process")
	viper.BindPFlag("nice", rootCmd.PersistentFlags().Lookup("nice"))

	var cancelCtx context.Context
	cancelCtx, cancelFunc = context.WithCancel(context.Background())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-stop
		log.Info().Str("signal", sig.String()).Msg("stopping")
		cancelFunc()
	}()

	err = rootCmd.ExecuteContext(cancelCtx)
	if err != nil {
		log.Err(err).Msg("command failed")
		cancelFunc()
		return failureCode
	}

	return 0
}

//--------------------------------------------------------------------------------
// private

const logDstLabel = "log-dst"
const minimalTimeFormat = "15:04:05.000"

var failureCode = 1
var initialized = false

var configPath = "$HOME/." + buildinfo.App.Name
var dumpConfig = false
var logF *os.File

var cancelFunc func()
var pidPath *pidpath.PidPath
var ipcServer *ipc.IPCServer

var rootCmd = &cobra.Command{
	Use:               buildinfo.App.Name,
	Short:             buildinfo.App.Description,
	Version:           buildinfo.All,
	SilenceUsage:      true,
	PersistentPreRunE: atStart,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if dumpConfig {
			return dump("config", cmd.OutOrStdout())
		}
		return cmd.Help()
	},
}

func atStart(cmd *cobra.Command, _ []string) error {
	if initialized {
		return nil
	}

	initialized = true
	pidPath = pidpath.NewPidPath(viper.GetString("pidpath"), 0666)
	ipcServer = &ipc.IPCServer{}

	viper.SetConfigName(filepath.Base(configPath))
	viper.SetConfigType("toml")
	viper.AddConfigPath(filepath.Dir(configPath))

	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("unable to read config file: %w", err)
		}
	} else {
		viper.OnConfigChange(func(e fsnotify.Event) {
			confLogLevel := viper.GetString("log-level")
			level, err := zerolog.ParseLevel(confLogLevel)
			if err != nil {
				log.Err(err).Str("level", confLogLevel).Msg("unable to parse new log level")
			} else {
				zerolog.SetGlobalLevel(level)
			}
		})

		viper.WatchConfig()
	}

	err = setupLogging(cmd, "")
	if err != nil {
		return err
	}

	log.Debug().Str("file", viper.ConfigFileUsed()).Msg("config")
	return nil
}

func atExit() {
	if ipcServer != nil {
		ipcServer.Stop()
	}

	if logF != nil {
		logF.Close()
	}

	if pidPath != nil {
		pidPath.Release()
	}
}

func setupLogging(cmd *cobra.Command, logDst string) error {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	var logWriter io.Writer

	withTime := true

	if logDst == "" {
		logDst = viper.GetString(logDstLabel)
	}

	switch logDst {
	case "syslog":
		syslogger, err := syslog.New(syslog.LOG_INFO, buildinfo.App.Name)
		if err != nil {
			newErr := setupLogging(cmd, "stderr")
			if newErr != nil {
				return newErr
			}

			log.Warn().Err(err).Msg("unable to use syslog: switched to stderr")
			return nil
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
