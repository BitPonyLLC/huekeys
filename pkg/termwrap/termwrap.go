// Package termwrap is a helper to wrap long longs of text broken up at word
// boundaries based on the size of the parent terminal.
package termwrap

import (
	"fmt"
	"strings"

	"golang.org/x/term"

	"github.com/mitchellh/go-wordwrap"
)

// TermWrap manages the current size of the parent terminal.
type TermWrap struct {
	width  int
	height int
}

// NewTermWrap creates a new TermWrap with the size of the parent terminal or
// default values if the size is unknown.
func NewTermWrap(defaultWidth, defaultHeight int) *TermWrap {
	var err error
	tw := &TermWrap{}

	tw.width, tw.height, err = term.GetSize(0)
	if err != nil {
		tw.width = defaultWidth
		tw.height = defaultHeight
	}

	return tw
}

// Paragraph wraps the content at word boundaries.
func (tw *TermWrap) Paragraph(content string) string {
	return wordwrap.WrapString(content, uint(tw.width))
}

// IndentedParagraph wraps the content at word boundaries indent by the prefix
// if-and-only-if the current terminal width is at _least_ of a minimumWidth. If
// it is not, then it does not indent the content and makes full use of the
// width.
func (tw *TermWrap) IndentedParagraph(prefix, content string, minimumWidth int) string {
	width := tw.width
	if width > minimumWidth {
		width -= len(prefix) * 2
	}

	paragraph := wordwrap.WrapString(content, uint(width))

	if width != tw.width {
		lines := strings.Split(paragraph, "\n")
		for _, line := range lines {
			paragraph += fmt.Sprintf("%s%s\n", prefix, line)
		}
	}

	return paragraph
}
