package menu

import (
	"context"
	"strings"

	"github.com/getlantern/systray"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type item struct {
	sysItem *systray.MenuItem
	args    []string
}

func (it *item) run(ctx context.Context, log *zerolog.Logger, cmd *cobra.Command) {
	if log.GetLevel() < zerolog.InfoLevel {
		log.Debug().Str("cmd", strings.Join(it.args, " ")).Msg("executing")
	}

	cmd.SetArgs(it.args)
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		log.Error().Err(err).Str("cmd", it.args[0]).Msg("command failed")
	}
}
