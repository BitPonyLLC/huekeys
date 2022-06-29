package patterns

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"unicode"

	"github.com/BitPonyLLC/huekeys/internal/image_matcher"
	"github.com/BitPonyLLC/huekeys/pkg/keyboard"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/rs/zerolog"
)

// DesktopPattern is used when setting colors according to the dominant color of
// the active Gnome Desktop background picture.
type DesktopPattern struct {
	BasePattern

	env               *preservedEnv
	backgroundProcess *os.Process
}

var _ Pattern = (*DesktopPattern)(nil)  // ensures we conform to the Pattern interface
var _ runnable = (*DesktopPattern)(nil) // ensures we conform to the runnable interface

// DesktopPatternEnv returns an encoded environment variable needed to be passed
// along when run as root to preserve access to to the parent user's desktop
// configuration (gsettings).
func DesktopPatternEnv() (string, error) {
	pe := preservedEnv{
		User:       os.Getenv(userKey),
		RuntimeDir: os.Getenv(runtimeDirKey),
	}

	var key string
	if pe.User == "" {
		key = userKey
	}

	if pe.RuntimeDir == "" {
		key = runtimeDirKey
	}

	var keyErr error
	if key != "" {
		keyErr = fmt.Errorf("DesktopPattern requires %s to be set", key)
	}

	jVal, err := json.Marshal(pe)
	if err == nil {
		err = keyErr
	}

	sVal := base64.RawStdEncoding.EncodeToString([]byte(jVal))

	return desktopPatternKey + "=" + sVal, err
}

// SetEnv is invoked when the DesktopPattern needs additional environment values
// to pass along to the gsettings process (e.g. to ensure it monitors the right
// user desktop).
func (p *DesktopPattern) SetEnv(env string) error {
	p.env = &preservedEnv{}

	if env == "" {
		return nil
	}

	if strings.HasPrefix(env, desktopPatternKey) {
		env = env[len(desktopPatternKey)+1:]
	}

	jVal, err := base64.RawStdEncoding.DecodeString(env)
	if err != nil {
		return fmt.Errorf("unable to decode %s (%s): %w", desktopPatternKey, env, err)
	}

	err = json.Unmarshal(jVal, p.env)
	if err != nil {
		return fmt.Errorf("unable to unmarshal %s (%s): %w", desktopPatternKey, env, err)
	}

	p.log.Debug().Interface("env", p.env).Msg("using")
	return nil
}

//--------------------------------------------------------------------------------
// private

type preservedEnv struct {
	User       string
	RuntimeDir string
}

const desktopPatternKey = "DESKTOP_PATTERN"
const userKey = "USER"
const runtimeDirKey = "XDG_RUNTIME_DIR"

var pictureURIMonitorRE = regexp.MustCompile(`^\s*picture-uri(?:-dark)?:\s*'([^']+)'\s*$`)

func init() {
	register("desktop", &DesktopPattern{}, 0)
}

func (p *DesktopPattern) run() error {
	if p.env == nil {
		p.SetEnv(os.Getenv(desktopPatternKey))
	}

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

	err = p.setColorFrom(pictureURL.Path)
	if err != nil {
		return err
	}

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
	p.log.Debug().Int("pid", p.backgroundProcess.Pid).Str("cmd", cmd.String()).Msg("started")

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

func (p *DesktopPattern) getDesktopSetting(group, key string) (string, error) {
	val, err := p.newDesktopSettingCmd("get", group, key).Output()
	if err != nil {
		p.log.Err(err).Str("group", group).Str("key", key).Msg("can't get setting value")
		return "", err
	}

	val = bytes.TrimFunc(val, func(r rune) bool { return unicode.IsSpace(r) || r == '\'' })
	return string(val), nil
}

func (p *DesktopPattern) newDesktopSettingCmd(action, group, key string) *exec.Cmd {
	fullGroup := "org.gnome.desktop." + group
	cmd := "gsettings"
	args := []string{action, fullGroup, key}

	if p.env.User != "" {
		shCmd := p.env.String() + cmd + " " + strings.Join(args, " ")
		cmd = "sudo"
		args = []string{"-u", p.env.User, "sh", "-c", shCmd}
	}

	return exec.Command(cmd, args...)
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
			p.log.Err(err).Int("pid", proc.Pid).Msg("can't kill desktop background monitor")
		}
	}
}

func (e *preservedEnv) String() string {
	str := ""

	if e.RuntimeDir != "" {
		str += runtimeDirKey + "=" + e.RuntimeDir + " "
	}

	return str
}
