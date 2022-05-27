package patterns

import (
	"context"

	"github.com/BitPonyLLC/huekeys/internal/menu"

	"github.com/rs/zerolog"
)

type WaitPattern struct {
	BasePattern

	Menu *menu.Menu
}

var _ Pattern = (*WaitPattern)(nil) // ensures we conform to the Pattern interface

// NewWaitPattern creates a special-purpose pattern that does nothing but wait forever.
func NewWaitPattern() *WaitPattern {
	return &WaitPattern{BasePattern: BasePattern{Name: "wait"}}
}

// SPECIAL CASE!! This _overrides_ BasePattern.Run() and will hang forever,
//                waiting for the parent context to interrupt.
func (p *WaitPattern) Run(parent context.Context, log *zerolog.Logger) error {
	if p.Menu == nil {
		<-parent.Done()
	} else {
		p.Menu.Show(parent, log)
	}

	return nil
}
