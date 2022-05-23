package buildinfo

import (
	_ "embed"
	"fmt"
	"runtime/debug"
)

const Name = "huekeys"
const Description = "Control the keyboard backlight on System76 laptops"

//go:generate sh -c "date -u +%Y-%m-%dT%H:%M:%SZ | tr -d '\n' > build_time.txt"

//go:embed build_time.txt
var BuildTime string

var CommitHash string

// FIXME: figure out why the latest tag doesn't show up in the build info below...
//go:generate sh -c "git describe --tags --abbrev=0 --dirty --always | tr -d '\n' > version.txt"

//go:embed version.txt
var Version string

var All string

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				CommitHash = setting.Value[0:7] // use the "short" hash
				break
			}
		}
	}

	All = fmt.Sprintf("%s (%s at %s)", Version, CommitHash, BuildTime)
}
