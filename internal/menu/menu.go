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

		for {
			cases := make([]reflect.SelectCase, len(m.items)+1)
			cases[0] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(quitItem.ClickedCh)}
			for i, it := range m.items {
				ch := it.sysItem.ClickedCh
				cases[i+1] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
			}

			// index, value, ok := ...
			// ok will be true if the channel has not been closed.
			index, _, _ := reflect.Select(cases)

			if cancelFunc != nil {
				// stop the active pattern
				cancelFunc()
			}

			if index == 0 {
				systray.Quit()
				return
			}

			cancelCtx, cancelFunc = context.WithCancel(m.ctx)
			it := m.items[index-1]
			go it.run(cancelCtx, m.log, m.Cmd)
		}
	}()
}
