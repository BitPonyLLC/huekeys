package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/BitPonyLLC/huekeys/buildinfo"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var dumpCmd = &cobra.Command{
	Use:    "dump",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, arg := range args {
			err := dump(arg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dumpCmd)
}

func dump(key string, writer io.Writer) error {
	val := ""

	switch key {
	case "config":
		return showConfig(writer)
	case "name":
		val = buildinfo.App.Name
	case "desc":
		val = buildinfo.App.Description
	case "full":
		val = buildinfo.App.FullDescription
	default:
		return fmt.Errorf("unknown dump key requested: %s", key)
	}

	_, err := writer.Write([]byte(val))
	if err != nil {
		return err
	}

	return nil
}

func showConfig(writer io.Writer) error {
	tf, err := os.CreateTemp(os.TempDir(), buildinfo.App.Name)
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
		fmt.Fprintln(writer, scanner.Text())
	}

	return nil
}
