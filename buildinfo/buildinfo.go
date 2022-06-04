package buildinfo

import (
	_ "embed"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type AppInfo struct {
	buildInfo

	Name            string `yaml:"name"`
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
	err := yaml.Unmarshal(app, &App)
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
