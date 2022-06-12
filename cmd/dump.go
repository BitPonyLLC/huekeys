package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"

	"github.com/BitPonyLLC/huekeys/buildinfo"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cpuFile *os.File

var dumpCmd = &cobra.Command{
	Use:    "dump",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, arg := range args {
			err := dump(cmd, arg)
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

func dump(cmd *cobra.Command, key string) error {
	val := ""

	switch key {
	case "config":
		return showConfig(cmd.OutOrStdout())
	case "name":
		val = buildinfo.App.Name
	case "desc":
		val = buildinfo.App.Description
	case "full":
		val = buildinfo.App.FullDescription
	case "mem":
		if ok, err := considerRemote(cmd); ok || err != nil {
			return err
		}
		f, err := createFile("mem")
		if err != nil {
			return err
		}
		defer f.Close()
		runtime.GC()
		if err = pprof.WriteHeapProfile(f); err != nil {
			return fmt.Errorf("failed to write heap profile to %s: %w", f.Name(), err)
		}
		val = f.Name()
	case "cpu-start":
		if ok, err := considerRemote(cmd); ok || err != nil {
			return err
		}
		if cpuFile != nil {
			return fmt.Errorf("CPU profile already writing to %s", cpuFile.Name())
		}
		var err error
		if cpuFile, err = createFile("cpu"); err != nil {
			return err
		}
		if err = pprof.StartCPUProfile(cpuFile); err != nil {
			cpuFile.Close()
			return fmt.Errorf("failed to start CPU profile: %w", err)
		}
		val = "started"
	case "cpu-stop":
		if ok, err := considerRemote(cmd); ok || err != nil {
			return err
		}
		if cpuFile == nil {
			return fmt.Errorf("CPU profile not running")
		}
		pprof.StopCPUProfile()
		val = cpuFile.Name()
		cpuFile.Close()
		cpuFile = nil
	default:
		return fmt.Errorf("unknown dump key requested: %s", key)
	}

	cmd.Print(val)
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

func considerRemote(cmd *cobra.Command) (bool, error) {
	if waitPidPath.IsRunning() && !waitPidPath.IsOurs() {
		return true, sendViaIPC(cmd)
	}

	return false, nil
}

func createFile(ext string) (*os.File, error) {
	fn := filepath.Join(os.TempDir(), fmt.Sprint(buildinfo.App.Name, "-", os.Getpid(), ".", ext))
	f, err := os.Create(fn)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", fn, err)
	}

	return f, nil
}
