package menu

import (
	"context"
	_ "embed"
	"os/exec"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/BitPonyLLC/huekeys/pkg/ipc"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/getlantern/systray"
	"github.com/rs/zerolog"
)

const brightnessPrefix = "Brightness: "
const colorPrefix = "Color: "

//go:embed tray_icon.png
var trayIcon []byte

type Menu struct {
	PatternName string

	ctx      context.Context
	log      *zerolog.Logger
	client   *ipc.Client
	sockpath string

	// not using a map because we want order preserved
	items   []*item
	checked *item
	errored *item

	errMsg     string
	brightness string
	color      string

	errParentItem *systray.MenuItem
	errMsgItem    *systray.MenuItem

	infoItem       *systray.MenuItem
	brightnessItem *systray.MenuItem
	colorItem      *systray.MenuItem

	pauseItem *item
	offItem   *item
	quitItem  *systray.MenuItem
}

type item struct {
	name    string
	msg     string
	sysItem *systray.MenuItem
}

const (
	done = iota
	errParent
	errMsg
	info
	brightness
	color
	pause
	off
	quit

	// finally, indicate the last explicit item
	end
)

// Add will create a menu item with the provided name displayed and will send
// the provided msg over the IPC client.
func (m *Menu) Add(name string, msg string) {
	m.items = append(m.items, &item{
		name:    name,
		msg:     msg,
		sysItem: systray.AddMenuItemCheckbox(title(name), "", false),
	})
}

// Show will display the menu in the system tray and block until quit or parent
// is canceled.
func (m *Menu) Show(ctx context.Context, log *zerolog.Logger, sockPath string) error {
	m.ctx = ctx
	m.log = log
	m.sockpath = sockPath

	systray.SetIcon(trayIcon)

	systray.AddSeparator()
	m.errParentItem = systray.AddMenuItem("Error", "")
	m.errParentItem.Hide()
	m.errMsgItem = m.errParentItem.AddSubMenuItemCheckbox("", "", false)

	systray.AddSeparator()
	m.infoItem = systray.AddMenuItem("Info", "")
	m.brightnessItem = m.infoItem.AddSubMenuItemCheckbox(brightnessPrefix+"🯄", "", false)
	m.colorItem = m.infoItem.AddSubMenuItemCheckbox(colorPrefix+"🯄", "", false)

	systray.AddSeparator()
	m.pauseItem = &item{
		sysItem: systray.AddMenuItemCheckbox("Pause", "", false),
		msg:     "stop",
	}
	m.offItem = &item{
		sysItem: systray.AddMenuItemCheckbox("Off", "", false),
		msg:     "stop --off",
	}

	systray.AddSeparator()
	m.quitItem = systray.AddMenuItem("Quit", "")

	go m.watch()
	defer func() { m.client.Close() }() // can't immediately defer m.client.Close since it's not set

	if m.PatternName != "" {
		_, err := ipc.Send(m.sockpath, "run "+m.PatternName)
		if err != nil {
			return err
		}
	}

	systray.Run(m.listen, nil)
	return nil
}

//--------------------------------------------------------------------------------
// private

func (m *Menu) watch() {
	defer util.LogRecover()

	m.check(m.pauseItem) // assume not running until proven otherwise, below...

	m.client = &ipc.Client{
		Foreground: true,
		RespCB: func(line string) bool {
			key, value, found := strings.Cut(line, ":")
			if !found {
				m.log.Warn().Str("line", line).Msg("ignoring unknown watch result")
				return true
			}

			value = strings.TrimSpace(value)

			brightness := ""
			color := ""
			running := ""

			switch key {
			case "b":
				brightness = value
			case "c":
				color = value
			case "r":
				running = value
			default:
				m.log.Warn().Str("line", line).Msg("ignoring unknown watch result key")
			}

			m.update(brightness, color, running)
			return true
		},
	}

	err := m.client.Send(m.sockpath, "run watch")
	if err != nil {
		m.log.Err(err).Msg("unable to run watch")
	}
}

func (m *Menu) listen() {
	defer func() {
		util.LogRecover()
		ipc.Send(m.sockpath, "quit")
		systray.Quit()
	}()

	cases := make([]reflect.SelectCase, len(m.items)+end)

	for {
		// explicit channels
		cases[done] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.ctx.Done())}

		cases[errParent] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.errParentItem.ClickedCh)}
		cases[errMsg] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.errMsgItem.ClickedCh)}

		cases[info] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.infoItem.ClickedCh)}
		cases[brightness] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.brightnessItem.ClickedCh)}
		cases[color] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.colorItem.ClickedCh)}

		cases[pause] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.pauseItem.sysItem.ClickedCh)}
		cases[off] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.offItem.sysItem.ClickedCh)}
		cases[quit] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.quitItem.ClickedCh)}

		// dynamic channels
		for i, it := range m.items {
			ch := it.sysItem.ClickedCh
			cases[i+end] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
		}

		// wait for something to be sent...
		index, _, ok := reflect.Select(cases)

		var it *item
		if index < end {
			switch index {
			case done:
				return
			case pause:
				it = m.pauseItem
			case off:
				it = m.offItem
			case quit:
				return
			case errParent:
				// ignore
			case errMsg:
				m.clip(m.errMsg)
				m.clearErr()
				m.errMsgItem.Uncheck()
			case info:
			case brightness:
				m.clip(m.brightness)
				m.brightnessItem.Uncheck()
			case color:
				m.clip(m.color)
				m.colorItem.Uncheck()
			default:
				m.log.Fatal().Int("index", index).Msg("missing channel handler")
				return
			}

			if it == nil {
				continue
			}
		}

		if !ok {
			continue
		}

		if it == nil {
			index -= end // adjust for explicit channels
			it = m.items[index]
		}

		m.log.Debug().Str("cmd", it.msg).Msg("sending")

		resp, err := ipc.Send(m.sockpath, it.msg)
		if err != nil {
			m.markAndShowErr(err, it)
		} else {
			m.check(it)
		}

		if resp == "" {
			continue
		}

		var ev *zerolog.Event
		if strings.HasPrefix("ERR:", resp) {
			ev = m.log.Error()
		} else {
			ev = m.log.Debug()
		}

		ev.Str("cmd", it.msg).Str("resp", resp).Msg("received a response")
	}
}

func (m *Menu) check(it *item) {
	if m.checked != nil {
		m.checked.sysItem.Uncheck()
	}

	it.sysItem.Check()
	m.checked = it
}

func (m *Menu) update(brightness, color, running string) {
	if brightness != "" {
		m.brightness = brightness
		m.brightnessItem.SetTitle(brightnessPrefix + brightness)
		bn, _ := strconv.Atoi(brightness)
		if bn == 0 {
			m.check(m.offItem)
		} else {
			m.offItem.sysItem.Uncheck()
		}
	}

	if color != "" {
		m.color = color
		m.colorItem.SetTitle(colorPrefix + color)
	}

	if running != "" {
		m.pauseItem.sysItem.Uncheck()

		if m.checked != nil {
			m.checked.sysItem.Uncheck()
			m.checked = nil
		}

		for _, it := range m.items {
			if it.name == running {
				m.check(it)
				break
			}
		}

		if m.checked == nil {
			m.log.Warn().Str("running", running).Msg("active pattern was not found in menu items")
		}
	}
}

func (m *Menu) clip(content string) {
	cmd := exec.Command("xclip", "-sel", "clip", "-i")
	writer, err := cmd.StdinPipe()
	if err != nil {
		m.log.Err(err).Msg("unable to open stdin pipe for xclip")
		return
	}

	err = cmd.Start()
	if err != nil {
		m.log.Err(err).Msg("unable to start xclip")
		return
	}
	defer cmd.Process.Release()

	_, err = writer.Write([]byte(content))
	if err != nil {
		m.log.Err(err).Msg("unable to write content to xclip")
		return
	}

	writer.Close()

	err = cmd.Wait()
	if err != nil {
		m.log.Err(err).Msg("unable to wait for xclip to exit")
		return
	}

	m.log.Trace().Str("content", content).Msg("saved to xclip")
}

func (m *Menu) markAndShowErr(err error, it *item) {
	m.clearErr()
	m.log.Err(err).Str("cmd", it.msg).Msg("sending")
	it.sysItem.SetTitle("❌ " + title(it.name))
	m.showErr(err)
}

func (m *Menu) showErr(err error) {
	if err == nil {
		return
	}

	m.errMsg = err.Error()
	m.errMsgItem.SetTitle(m.errMsg)
	m.errParentItem.Show()
}

func (m *Menu) clearErr() {
	it := m.errored
	if it == nil {
		return
	}

	m.errored = nil
	m.errMsg = ""
	m.errParentItem.Hide()
	m.errMsgItem.SetTitle(m.errMsg)

	// reset menu item title
	it.sysItem.SetTitle(title(it.name))
}

func title(name string) string {
	return cases.Title(language.English).String(name)
}
