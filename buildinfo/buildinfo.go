package buildinfo

import (
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// AppInfo provides static data about the running application
type AppInfo struct {
	buildInfo

	ExePath string `yaml:"-"`

	Name            string `yaml:"name"`
	URL             string `yaml:"url"`
	ReverseDNS      string `yaml:"reverse_dns"`
	Vendor          string `yaml:"vendor"`
	Description     string `yaml:"description"`
	FullDescription string `yaml:"full_description"`
}

type buildInfo struct {
	Version    string    `yaml:"version"`
	CommitHash string    `yaml:"commit_hash"`
	BuildTime  time.Time `yaml:"build_time"`
}

var App AppInfo
var All string

//go:generate make -C .. buildinfo

//go:embed app.yml
var app []byte

//go:embed build.yml
var build []byte

func init() {
	var err error

	App.ExePath, err = os.Executable()
	if err != nil {
		log.Fatal().Err(err).Msg("unable to determine executable pathname")
	}

	err = yaml.Unmarshal(app, &App)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse embedded app info")
	}

	err = yaml.Unmarshal(build, &App.buildInfo)
	if err != nil {
		log.Fatal().Err(err).Msg("unable to parse embedded build info")
	}

	buildTime := App.BuildTime.Format(time.RFC3339)
	All = fmt.Sprintf("%s (%s at %s)", App.Version, App.CommitHash, buildTime)
}
