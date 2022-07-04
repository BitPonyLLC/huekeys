package util

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/BitPonyLLC/huekeys/buildinfo"

	"github.com/rs/zerolog/log"
)

func Extract(pathname string, content []byte, data any) error {
	file, err := os.OpenFile(pathname, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", pathname, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("unable to stat %s: %w", pathname, err)
	}

	if stat.Size() > 0 {
		exeStat, err := os.Stat(buildinfo.App.ExePath)
		if err != nil {
			return fmt.Errorf("unable to stat %s: %w", buildinfo.App.ExePath, err)
		}

		if exeStat.ModTime().Before(stat.ModTime()) {
			log.Debug().Str("path", pathname).Msg("unchanged")
			return nil
		}
	}

	err = file.Truncate(0)
	if err != nil {
		return fmt.Errorf("unable to truncate %s: %w", pathname, err)
	}

	name := filepath.Base(pathname)
	if data == nil {
		_, err = file.Write(content)
		if err != nil {
			return fmt.Errorf("unable to write %s content: %w", name, err)
		}
	} else {
		tmpl, err := template.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("unable to parse %s template: %w", name, err)
		}

		err = tmpl.Execute(file, data)
		if err != nil {
			return fmt.Errorf("unable to execute %s template: %w", name, err)
		}
	}

	log.Debug().Str("path", pathname).Msg("updated")
	return nil
}
