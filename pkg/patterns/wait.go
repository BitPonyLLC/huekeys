package patterns

import (
	"context"
	"time"

	"github.com/BitPonyLLC/huekeys/pkg/keyboard"

	"github.com/rs/zerolog"
)

// WaitPattern is a special kind of pattern that doesn't do anything, instead,
// waiting on external commands to be issued (by the menu or another cli) to
// trigger a pattern.
type WaitPattern struct {
	BasePattern
}

const MonitorLabel = "monitor"

var _ Pattern = (*WaitPattern)(nil) // ensures we conform to the Pattern interface

// Run is overriding the BasePattern version as a special case and will hang
// forever, waiting for the parent context to interrupt.
func (p *WaitPattern) Run(parent context.Context, _ *zerolog.Logger) error {
	monitorPeriod := p.getMonitorPeriod()
	if monitorPeriod > 0 {
		keyboard.StartMonitor(parent, monitorPeriod)
	}

	<-parent.Done()
	return nil
}

func init() {
	register("wait", &WaitPattern{}, 0)
}

func (p *WaitPattern) getMonitorPeriod() time.Duration {
	return config.GetDuration(p.Name + "." + MonitorLabel)
}
