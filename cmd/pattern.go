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
	addPatternCmd("wait for remote commands", patterns.Get("wait"))
	addPatternCmd("pulse the keyboard brightness up and down", patterns.Get("pulse"))
	addPatternCmd("loop through all the colors of the rainbow", patterns.Get("rainbow"))
	addPatternCmd("constantly change the color to a random selection", patterns.Get("random"))
	addPatternCmd("change the color according to CPU utilization (cold to hot)", patterns.Get("cpu"))
	addPatternCmd("monitor the desktop picture and change the keyboard color to match", patterns.Get("desktop"))

	//----------------------------------------
	typingPattern := patterns.Get("typing")
	typingLabel := typingPattern.GetBase().Name + "."

	typeCmd := addPatternCmd("change the color according to typing speed (cold to hot)", typingPattern)

	typeCmd.Flags().String(patterns.InputEventIDLabel, "", "input event ID to monitor")
	viper.BindPFlag(typingLabel+patterns.InputEventIDLabel, typeCmd.Flags().Lookup(patterns.InputEventIDLabel))

	typeCmd.Flags().Bool(patterns.AllKeysLabel, false, "count any key pressed instead of only those that are considered \"printable\"")
	viper.BindPFlag(typingLabel+patterns.AllKeysLabel, typeCmd.Flags().Lookup(patterns.AllKeysLabel))

	typeCmd.Flags().StringP(patterns.IdleLabel, "i", "", "name of pattern to run while keyboard is idle for more than the idle period")
	viper.BindPFlag(typingLabel+patterns.IdleLabel, typeCmd.Flags().Lookup(patterns.IdleLabel))

	typeCmd.Flags().DurationP(patterns.IdlePeriodLabel, "p", patterns.DefaultIdlePeriod, "amount of idle time to wait before starting the idle pattern")
	viper.BindPFlag(typingLabel+patterns.IdlePeriodLabel, typeCmd.Flags().Lookup(patterns.IdlePeriodLabel))
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
			return pattern.Run(cmd.Context(), &log.Logger)
		},
	}

	defaultDelay := pattern.GetDefaultDelay()
	if defaultDelay != 0 {
		cmd.Flags().DurationP("delay", "d", defaultDelay,
			"the amount of time to wait between updates (units: ns, us, ms, s, m, h)")
		viper.BindPFlag(basePattern.Name+".delay", cmd.Flags().Lookup("delay"))
	}

	runCmd.AddCommand(cmd)
	return cmd
}
