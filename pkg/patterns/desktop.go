package patterns

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"unicode"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var pictureURIMonitorRE = regexp.MustCompile(`^\s*picture-uri(?:-dark)?:\s*'([^']+)'\s*$`)
var backgroundProcess *os.Process

func MatchDesktopBackground(ctx context.Context) error {
	colorScheme, err := getDesktopSetting("interface", "color-scheme")
	if err != nil {
		return err
	}

	pictureKey := "picture-uri"
	if colorScheme == "prefer-dark" {
		pictureKey += "-dark"
	}

	pictureURIStr, err := getDesktopSetting("background", pictureKey)
	if err != nil {
		return err
	}

	pictureURL, err := url.Parse(pictureURIStr)
	if err != nil {
		return fmt.Errorf("can't parse picture URI (%s): %w", pictureURIStr, err)
	}

	setColorFrom(pictureURL.Path)

	cmd := newDesktopSettingCmd("monitor", "background", pictureKey)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("can't get stdout of desktop background monitor: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("can't start desktop background monitor: %w", err)
	}

	backgroundProcess = cmd.Process

	go func() {
		state, err := backgroundProcess.Wait()
		var ev *zerolog.Event
		if stopRequested {
			ev = log.Debug()
		} else {
			ev = log.Error()
		}
		ev.Err(err).Interface("state", state).Msg("desktop background monitor has stopped")
	}()

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			m := pictureURIMonitorRE.FindStringSubmatch(line)
			if m == nil || len(m) < 2 || m[1] == "" {
				log.Warn().Str("line", line).Msg("ignoring unknown content from desktop background monitor")
				continue
			}
			setColorFrom(m[1])
		}
	}()

	<-ctx.Done()
	stopDesktopBackgroundMonitor()

	return nil
}

func newDesktopSettingCmd(action, group, key string) *exec.Cmd {
	cmdName := "gsettings"
	args := []string{}

	// if running as root via sudo, need to ask for the user's desktop image...
	if os.Getuid() == 0 {
		sudoUser := os.Getenv("SUDO_USER")
		if sudoUser != "" {
			cmdName = "sudo"
			args = []string{"-Eu", sudoUser, "gsettings"}
			if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
				// we need access to the user's gnome session in order to look up correct setting values
				log.Fatal().Msg("running as root without user environment: add `-E` when invoking sudo")
			}
		}
	}

	group = "org.gnome.desktop." + group
	args = append(args, action, group, key)
	cmd := exec.Command(cmdName, args...)
	return cmd
}

func getDesktopSetting(group, key string) (string, error) {
	// TODO: consider using D-Bus directly instead of gsettings...
	val, err := newDesktopSettingCmd("get", group, key).Output()
	if err != nil {
		log.Error().Err(err).Str("group", group).Str("key", key).Msg("can't get setting value")
		return "", err
	}

	val = bytes.TrimFunc(val, func(r rune) bool { return unicode.IsSpace(r) || r == '\'' })
	return string(val), nil
}

func stopDesktopBackgroundMonitor() {
	if backgroundProcess != nil {
		p := backgroundProcess
		log.Debug().Int("pid", p.Pid).Msg("stopping desktop background monitor")
		backgroundProcess = nil
		stopRequested = true
		err := p.Kill()
		if err != nil {
			log.Error().Err(err).Int("pid", p.Pid).Msg("can't kill desktop background monitor")
		}
	}
}
