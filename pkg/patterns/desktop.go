package patterns

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"syscall"
	"unicode"

	"github.com/BitPonyLLC/huekeys/internal/image_matcher"
	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/rs/zerolog"
)

type DesktopPattern struct {
	BasePattern

	backgroundProcess *os.Process
}

func NewDesktopPattern() *DesktopPattern {
	p := &DesktopPattern{}
	p.BasePattern = BasePattern{
		Name: "desktop",
		self: p,
	}
	return p
}

var _ Pattern = (*DesktopPattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*DesktopPattern)(nil) // ensures we conform to the runnable interface

var pictureURIMonitorRE = regexp.MustCompile(`^\s*picture-uri(?:-dark)?:\s*'([^']+)'\s*$`)

func (p *DesktopPattern) run() error {
	colorScheme, err := p.getDesktopSetting("interface", "color-scheme")
	if err != nil {
		return err
	}

	pictureKey := "picture-uri"
	if colorScheme == "prefer-dark" {
		pictureKey += "-dark"
	}

	pictureURIStr, err := p.getDesktopSetting("background", pictureKey)
	if err != nil {
		return err
	}

	pictureURL, err := url.Parse(pictureURIStr)
	if err != nil {
		return fmt.Errorf("can't parse picture URI (%s): %w", pictureURIStr, err)
	}

	p.setColorFrom(pictureURL.Path)

	cmd := p.newDesktopSettingCmd("monitor", "background", pictureKey)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("can't get stdout of desktop background monitor: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("can't start desktop background monitor: %w", err)
	}

	p.backgroundProcess = cmd.Process
	p.log.Debug().Int("pid", p.backgroundProcess.Pid).Msg("started desktop background monitor")

	go func() {
		defer util.LogRecover()
		proc := p.backgroundProcess
		state, err := proc.Wait()
		var ev *zerolog.Event
		if p.stopRequested {
			ev = p.log.Debug()
		} else {
			ev = p.log.Error()
		}
		ev.Err(err).Int("pid", proc.Pid).Str("state", state.String()).
			Msg("desktop background monitor has stopped")
	}()

	go func() {
		defer util.LogRecover()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			m := pictureURIMonitorRE.FindStringSubmatch(line)
			if m == nil || len(m) < 2 || m[1] == "" {
				p.log.Warn().Str("line", line).Msg("ignoring unknown content from desktop background monitor")
				continue
			}
			p.setColorFrom(m[1])
		}
	}()

	<-p.ctx.Done()
	p.stopDesktopBackgroundMonitor()

	return nil
}

func (p *DesktopPattern) newDesktopSettingCmd(action, group, key string) *exec.Cmd {
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
				p.log.Fatal().Msg("running as root without user environment: add `-E` when invoking sudo")
			}
		}
	}

	group = "org.gnome.desktop." + group
	args = append(args, action, group, key)
	cmd := exec.Command(cmdName, args...)
	return cmd
}

func (p *DesktopPattern) getDesktopSetting(group, key string) (string, error) {
	// TODO: consider using D-Bus directly instead of gsettings...
	val, err := p.newDesktopSettingCmd("get", group, key).Output()
	if err != nil {
		p.log.Error().Err(err).Str("group", group).Str("key", key).Msg("can't get setting value")
		return "", err
	}

	val = bytes.TrimFunc(val, func(r rune) bool { return unicode.IsSpace(r) || r == '\'' })
	return string(val), nil
}

func (p *DesktopPattern) setColorFrom(u string) error {
	pictureURL, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("can't parse picture URI (%s): %w", pictureURL, err)
	}

	color, err := image_matcher.GetDominantColorOf(pictureURL.Path)
	if err != nil {
		return fmt.Errorf("can't determine dominant color: %w", err)
	}

	p.log.Info().Str("color", color).Str("path", pictureURL.Path).Msg("setting")

	return keyboard.ColorFileHandler(color)
}
func (p *DesktopPattern) stopDesktopBackgroundMonitor() {
	if p.backgroundProcess != nil {
		proc := p.backgroundProcess
		p.log.Debug().Int("pid", proc.Pid).Msg("stopping desktop background monitor")
		p.stopRequested = true
		err := syscall.Kill(-proc.Pid, syscall.SIGTERM)
		if err != nil {
			p.log.Error().Err(err).Int("pid", proc.Pid).Msg("can't kill desktop background monitor")
		}
	}
}
