package patterns

import (
	"context"

	"github.com/rs/zerolog"
)

type WaitPattern struct {
	BasePattern
}

var _ Pattern = (*WaitPattern)(nil) // ensures we conform to the Pattern interface

func NewWaitPattern() *WaitPattern {
	return &WaitPattern{BasePattern: BasePattern{Name: "wait"}}
}

// SPECIAL CASE!! This _overrides_ BasePattern.Run() and will hang forever,
//                waiting for the parent context to interrupt.
func (p *WaitPattern) Run(parent context.Context, _ *zerolog.Logger) error {
	<-parent.Done()
	return nil
}