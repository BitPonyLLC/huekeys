package menu

import (
	"context"
	_ "embed"
	"reflect"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/BitPonyLLC/huekeys/pkg/ipc"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/getlantern/systray"
	"github.com/rs/zerolog"
)

const maxErrorMsgLen = 70
const brightnessPrefix = "Brightness: "
const colorPrefix = "Color: "

//go:embed tray_icon.png
var trayIcon []byte

type Menu struct {
	PatternName string

	ctx      context.Context
	log      *zerolog.Logger
	sockpath string

	// not using a map because we want order preserved
	names   []string
	items   []*item
	checked *item

	errIndex      int
	errParentItem *systray.MenuItem
	errMsgItem    *systray.MenuItem

	infoItem       *systray.MenuItem
	brightnessItem *systray.MenuItem
	colorItem      *systray.MenuItem

	quitItem *systray.MenuItem
}

type item struct {
	sysItem *systray.MenuItem
	msg     string
}

const (
	done = iota
	errParent
	errMsg
	info
	brightness
	color
	quit

	// finally, indicate the last explicit item
	end
)

// Add will create a menu item with the provided name displayed and will send
// the provided msg over the IPC client.
func (m *Menu) Add(name string, msg string) {
	sysItem := systray.AddMenuItemCheckbox(title(name), "", false)
	m.names = append(m.names, name)
	m.items = append(m.items, &item{sysItem: sysItem, msg: msg})
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
	m.quitItem = systray.AddMenuItem("Quit", "")

	if m.PatternName != "" {
		_, err := ipc.Send(m.sockpath, "run "+m.PatternName)
		if err != nil {
			return err
		}
	}

	err := m.update()
	if err != nil {
		return err
	}

	systray.Run(m.listen, nil)
	return nil
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
		cases[quit] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.quitItem.ClickedCh)}

		// dynamic channels
		for i, it := range m.items {
			ch := it.sysItem.ClickedCh
			cases[i+end] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
		}

		// wait for something to be sent...
		index, _, ok := reflect.Select(cases)

		if index < end {
			switch index {
			case quit:
				return
			case done:
				return
			case errParent:
				// ignore
			case errMsg:
				// TODO: copy error into clipboard
				m.clearErr()
			case info:
				m.showErr(m.update())
			case brightness:
				// TODO: copy color into clipboard
			case color:
				// TODO: copy color into clipboard
			default:
				m.log.Fatal().Int("index", index).Msg("missing channel handler")
				return
			}
			continue
		}

		if !ok {
			continue
		}

		index -= end // adjust for explicit channels

		it := m.items[index]
		m.log.Debug().Str("cmd", it.msg).Msg("sending")

		resp, err := ipc.Send(m.sockpath, it.msg)
		if err != nil {
			m.markAndShowErr(err, index, it)
		} else {
			if m.checked != nil {
				m.checked.sysItem.Uncheck()
			}

			it.sysItem.Check()
			m.checked = it
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

func (m *Menu) update() error {
	resp, err := ipc.Send(m.sockpath, "get")
	if err != nil {
		return err
	}

	lines := strings.Split(resp, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, val, found := strings.Cut(line, " = ")
		if !found {
			m.log.Warn().Str("line", line).Msg("unable to parse get response")
			continue
		}

		switch key {
		case "running":
			if m.checked != nil {
				m.checked.sysItem.Uncheck()
				m.checked = nil
			}

			running, _, found := strings.Cut(val, " ")
			if !found || running == "" {
				m.log.Warn().Str("val", val).Msg("unable to parse running value")
				continue
			}

			for i, name := range m.names {
				if name == running {
					m.checked = m.items[i]
					m.checked.sysItem.Check()
					continue
				}
			}

			if m.checked == nil {
				m.log.Warn().Str("running", running).Msg("active pattern was not found in menu items")
			}
		case "brightness":
			m.brightnessItem.SetTitle(brightnessPrefix + val)
		case "color":
			m.colorItem.SetTitle(colorPrefix + val)
		default:
			m.log.Warn().Str("line", line).Msg("ignoring unknown info from get response")
		}
	}

	return nil
}

func (m *Menu) clearErr() {
	if m.errIndex < 0 {
		return
	}

	index := m.errIndex
	m.errIndex = -1

	m.errParentItem.Hide()
	m.errMsgItem.SetTitle("")

	// reset menu item title
	t := title(m.names[index])
	m.items[index].sysItem.SetTitle(t)
}

func (m *Menu) markAndShowErr(err error, index int, it *item) {
	if m.errIndex > -1 {
		m.clearErr()
	}

	m.log.Err(err).Str("cmd", it.msg).Msg("sending")
	m.errIndex = index
	name := m.names[index]
	it.sysItem.SetTitle("❌ " + title(name))

	m.showErr(err)
}

func (m *Menu) showErr(err error) {
	if err == nil {
		return
	}

	msg := err.Error()
	if len(msg) > maxErrorMsgLen {
		msg = msg[0:maxErrorMsgLen-1] + "…"
	}

	m.errMsgItem.SetTitle(msg)
	m.errParentItem.Show()
}

func title(name string) string {
	return cases.Title(language.English).String(name)
}
