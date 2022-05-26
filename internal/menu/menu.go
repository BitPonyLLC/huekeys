package menu

import (
	"context"

	"github.com/BitPonyLLC/huekeys/buildinfo"

	"github.com/getlantern/systray"
)

//  go:embed tray_icon.png
// var trayIcon []byte

type menu struct {
	ctx context.Context
}

// Show will display the menu in the system tray and block until quit or parent
// is canceled.
func Show(ctx context.Context) {
	m := &menu{ctx: ctx}
	systray.Run(m.onReady, nil)
}

func (m *menu) onReady() {
	// systray.SetIcon(trayIcon)
	systray.SetTitle(buildinfo.Name)
	systray.SetTooltip(buildinfo.Description)

	pokeItem := systray.AddMenuItem("Poke", "Tell me somethin' now!")
	quitItem := systray.AddMenuItem("Quit", "Stop the rockin'!")

	go func() {
		for {
			select {
			case <-m.ctx.Done():
				systray.Quit()
				return
			case <-quitItem.ClickedCh:
				systray.Quit()
				return
			case <-pokeItem.ClickedCh:
				println("BARF cllicked")
			}
		}
	}()
}
