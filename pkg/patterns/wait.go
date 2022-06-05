package patterns

import (
	"context"

	"github.com/rs/zerolog"
)

// WaitPattern is a special kind of pattern that doesn't do anything, instead,
// waiting on external commands to be issued (by the menu or another cli) to
// trigger a pattern.
type WaitPattern struct {
	BasePattern
}

var _ Pattern = (*WaitPattern)(nil) // ensures we conform to the Pattern interface

// Run is overriding the BasePattern version as a special case and will hang
// forever, waiting for the parent context to interrupt.
func (p *WaitPattern) Run(parent context.Context, _ *zerolog.Logger) error {
	<-parent.Done()
	return nil
}

func init() {
	register("wait", &WaitPattern{}, 0)
}
