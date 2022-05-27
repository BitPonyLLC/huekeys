package menu

import (
	"context"
	_ "embed"
	"reflect"

	"github.com/getlantern/systray"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:embed tray_icon.png
var trayIcon []byte

type Menu struct {
	Cmd *cobra.Command

	ctx   context.Context
	log   *zerolog.Logger
	items []*item
}

const (
	quit = iota
	done
	end
)

func (m *Menu) Add(name string, args []string) {
	menuName := cases.Title(language.English).String(name)
	sysItem := systray.AddMenuItem(menuName, "")
	m.items = append(m.items, &item{sysItem: sysItem, args: args})
}

// Show will display the menu in the system tray and block until quit or parent
// is canceled.
func (m *Menu) Show(ctx context.Context, log *zerolog.Logger) {
	m.ctx = ctx
	m.log = log
	systray.Run(m.onReady, nil)
}

func (m *Menu) onReady() {
	systray.SetIcon(trayIcon)

	quitItem := systray.AddMenuItem("Quit", "Stop operations and exit")

	go func() {
		var cancelCtx context.Context
		var cancelFunc func()

		cases := make([]reflect.SelectCase, len(m.items)+end)
		for {
			// explicit channels to watch
			cases[quit] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(quitItem.ClickedCh)}
			cases[done] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(m.ctx.Done())}

			// dynamic channels based on items added
			for i, it := range m.items {
				ch := it.sysItem.ClickedCh
				cases[i+end] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
			}

			// ok will be true if the channel has not been closed
			index, _, ok := reflect.Select(cases)

			if cancelFunc != nil {
				// stop the active pattern
				cancelFunc()
			}

			switch index {
			case quit:
				systray.Quit()
				return
			case done:
				systray.Quit()
				return
			default:
				if ok {
					cancelCtx, cancelFunc = context.WithCancel(m.ctx)
					it := m.items[index-end]
					go it.run(cancelCtx, m.log, m.Cmd)
				}
			}
		}
	}()
}
