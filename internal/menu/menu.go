package menu

import (
	"context"
	_ "embed"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/BitPonyLLC/huekeys/pkg/ipc"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/getlantern/systray"
	"github.com/rs/zerolog"
)

const maxErrorMsgLen = 70

//go:embed tray_icon.png
var trayIcon []byte

type Menu struct {
	ctx context.Context
	log *zerolog.Logger
	cli *ipc.IPCClient

	// not using a map because we want order preserved
	names   []string
	items   []*item
	checked *item

	errIndex      int
	errParentItem *systray.MenuItem
	errMsgItem    *systray.MenuItem
}

type item struct {
	sysItem *systray.MenuItem
	msg     string
}

const (
	done = iota
	quit
	errParent
	errMsg

	// finally, indicate the last explicit item
	end
)

var getRunningRE = regexp.MustCompile(`running = (\S+)`)

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

	m.cli = &ipc.IPCClient{}
	err := m.cli.Connect(sockPath)
	if err != nil {
		return err
	}

	resp, err := m.cli.Send("get")
	if err != nil {
		return err
	}

	match := getRunningRE.FindStringSubmatch(resp)
	if len(match) == 2 {
		running := match[1]
		for i, name := range m.names {
			if name == running {
				m.checked = m.items[i]
				m.checked.sysItem.Check()
				break
			}
		}

		if m.checked == nil {
			m.log.Warn().Str("running", running).Msg("active pattern was not found in menu items")
		}
	}

	systray.Run(m.onReady, nil)
	return nil
}

func (m *Menu) onReady() {
	systray.SetIcon(trayIcon)

	systray.AddSeparator()
	m.errParentItem = systray.AddMenuItem("Error", "")
	m.errParentItem.Hide()
	m.errMsgItem = m.errParentItem.AddSubMenuItemCheckbox("", "", false)

	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "")

	go m.listen(quitItem.ClickedCh)
}

func (m *Menu) listen(quitCh chan struct{}) {
	defer func() {
		util.LogRecover()
		m.cli.Send("quit")
		systray.Quit()
	}()

	cases := make([]reflect.SelectCase, len(m.items)+end)

	for {
		// explicit channels
		cases[done] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.ctx.Done())}
		cases[quit] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(quitCh)}
		cases[errParent] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.errParentItem.ClickedCh)}
		cases[errMsg] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.errMsgItem.ClickedCh)}

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
				continue // ignore
			case errMsg:
				m.clearErr()
				continue // ignore
			default:
				m.log.Fatal().Int("index", index).Msg("missing channel handler")
				return
			}
		}

		if !ok {
			continue
		}

		index -= end // adjust for explicit channels

		it := m.items[index]
		m.log.Debug().Str("cmd", it.msg).Msg("sending")

		resp, err := m.cli.Send(it.msg)
		if err != nil {
			m.showErr(err, index, it)
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

func (m *Menu) showErr(err error, index int, it *item) {
	if m.errIndex > -1 {
		m.clearErr()
	}

	m.log.Error().Err(err).Str("cmd", it.msg).Msg("sending")
	m.errIndex = index
	name := m.names[index]
	it.sysItem.SetTitle("❌ " + title(name))

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
