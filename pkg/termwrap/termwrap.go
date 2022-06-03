package termwrap

import (
	"fmt"
	"strings"

	"github.com/mitchellh/go-wordwrap"
	"golang.org/x/term"
)

type TermWrap struct {
	width  int
	height int
}

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

func (tw *TermWrap) Paragraph(content string) string {
	return wordwrap.WrapString(content, uint(tw.width))
}

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
