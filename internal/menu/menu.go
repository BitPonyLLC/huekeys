package menu

import (
	"context"
	_ "embed"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/BitPonyLLC/huekeys/pkg/ipc"
	"github.com/BitPonyLLC/huekeys/pkg/util"

	"github.com/getlantern/systray"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

//go:embed tray_icon.png
var trayIcon []byte

type Menu struct {
	Cmd *cobra.Command

	ctx context.Context
	log *zerolog.Logger
	cli *ipc.IPCClient

	// not using a map because we want order preserved
	names   []string
	items   []*item
	checked *item
}

type item struct {
	sysItem *systray.MenuItem
	msg     string
}

const (
	quit = iota
	done
	end
)

var getRunningRE = regexp.MustCompile(`running = (\S+)`)

// Add will create a menu item with the provided name displayed and will send
// the provided msg over the IPC client.
func (m *Menu) Add(name string, msg string) {
	menuName := cases.Title(language.English).String(name)
	sysItem := systray.AddMenuItemCheckbox(menuName, "", false)
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
	if len(match) != 2 {
		return fmt.Errorf("unable to parse get response: %s", resp)
	}

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
		m.checked = m.items[0]
	}

	systray.Run(m.onReady, nil)
	return nil
}

func (m *Menu) onReady() {
	systray.SetIcon(trayIcon)
	systray.AddSeparator()
	quitItem := systray.AddMenuItem("Quit", "Stop operations and exit")
	go m.listen(quitItem.ClickedCh)
}

func (m *Menu) listen(quitCh chan struct{}) {
	defer util.LogRecover()
	defer systray.Quit()

	cases := make([]reflect.SelectCase, len(m.items)+end)

	for {
		// explicit channels
		cases[quit] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(quitCh)}
		cases[done] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.ctx.Done())}

		// dynamic channels
		for i, it := range m.items {
			ch := it.sysItem.ClickedCh
			cases[i+end] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
		}

		// wait for one!
		index, _, ok := reflect.Select(cases)

		switch index {
		case quit:
			return
		case done:
			return
		default:
			if !ok {
				continue
			}

			it := m.items[index-end]
			m.log.Debug().Str("cmd", it.msg).Msg("sending")

			resp, err := m.cli.Send(it.msg)
			if err != nil {
				// TODO: consider adding a "status" menu item (disabled/unclickable) to indicate problem!
				// modify name element with an exclamation character??
				// change icon, too!
				m.log.Error().Err(err).Str("cmd", it.msg).Msg("sending")
			} else {
				m.checked.sysItem.Uncheck()
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
}
