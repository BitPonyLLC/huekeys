package cmd

import (
	"github.com/BitPonyLLC/huekeys/pkg/patterns"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "runs a backlight pattern",
}

func init() {
	rootCmd.AddCommand(runCmd)

	//----------------------------------------
	addPatternCmd("wait for remote commands", patterns.NewWaitPattern())
	addPatternCmd("pulse the keyboard brightness up and down", patterns.NewPulsePattern())
	addPatternCmd("loop through all the colors of the rainbow", patterns.NewRainbowPattern())
	addPatternCmd("constantly change the color to a random selection", patterns.NewRandomPattern())
	addPatternCmd("change the color according to CPU utilization (cold to hot)", patterns.NewCPUPattern())
	addPatternCmd("monitor the desktop picture and change the keyboard color to match", patterns.NewDesktopPattern())

	//----------------------------------------
	typingPattern := patterns.NewTypingPattern()
	typingPatternCmd := addPatternCmd("change the color according to typing speed (cold to hot)", typingPattern)

	typingPatternCmd.Flags().String("input-event-id", typingPattern.InputEventID, "input event ID to monitor")
	viper.BindPFlag("typing.input-event-id", typingPatternCmd.Flags().Lookup("input-event-id"))

	typingPatternCmd.Flags().Bool("all-keys", typingPattern.CountAllKeys, "count any key pressed instead of only those that are considered \"printable\"")
	viper.BindPFlag("typing.all-keys", typingPatternCmd.Flags().Lookup("all-keys"))

	typingPatternCmd.Flags().StringP("idle", "i", "", "name of pattern to run while keyboard is idle for more than the idle period")
	viper.BindPFlag("typing.idle", typingPatternCmd.Flags().Lookup("idle"))

	typingPatternCmd.Flags().DurationP("idle-period", "p", typingPattern.IdlePeriod, "amount of idle time to wait before starting the idle pattern")
	viper.BindPFlag("typing.idle-period", typingPatternCmd.Flags().Lookup("idle-period"))

	typingPatternCmd.Args = func(cmd *cobra.Command, _ []string) (err error) {
		typingPattern.InputEventID = viper.GetString("typing.input-event-id")
		typingPattern.CountAllKeys = viper.GetBool("typing.all-keys")
		typingPattern.IdlePattern, err = getIdlePattern(cmd, viper.GetString("typing.idle"))
		return
	}
}

func addPatternCmd(short string, pattern patterns.Pattern) *cobra.Command {
	priority := viper.GetInt("nice")
	basePattern := pattern.GetBase()

	cmd := &cobra.Command{
		Use:   pattern.GetBase().Name,
		Short: short,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := pidPath.CheckAndSet(); err != nil {
				if pidPath.IsRunning() {
					if cmd.Name() == "wait" {
						return err
					}
					log.Debug().Err(err).Msg("ignoring")
					return nil
				}
				return fail(11, err)
			}
			if err := util.BeNice(priority); err != nil {
				return fail(12, err)
			}
			if _, ok := pattern.(*patterns.WaitPattern); ok {
				return ipcServer.Start(cmd.Context(), &log.Logger, viper.GetString("sockpath"), rootCmd)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if pidPath.IsRunning() && !pidPath.IsOurs() {
				return sendViaIPC(cmd)
			}
			basePattern.Delay = viper.GetDuration(basePattern.Name + ".delay")
			return pattern.Run(cmd.Context(), &log.Logger)
		},
	}

	if basePattern.Delay != 0 {
		cmd.Flags().DurationP("delay", "d", basePattern.Delay,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
		viper.BindPFlag(basePattern.Name+".delay", cmd.Flags().Lookup("delay"))
	}

	runCmd.AddCommand(cmd)
	return cmd
}

func getIdlePattern(cmd *cobra.Command, patternName string) (patterns.Pattern, error) {
	switch patternName {
	case "":
		return nil, nil
	case "pulse":
		return patterns.NewPulsePattern(), nil
	case "rainbow":
		return patterns.NewRainbowPattern(), nil
	case "random":
		return patterns.NewRandomPattern(), nil
	case "cpu":
		return patterns.NewCPUPattern(), nil
	case "desktop":
		return patterns.NewDesktopPattern(), nil
	default:
		return nil, fail(13, "unknown pattern: %s", patternName)
	}
}
