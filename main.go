// Huekeys as a command line tool (CLI) is documented in the project's README:
// https://github.com/BitPonyLLC/huekeys#readme
package main

import (
	"os"

	"github.com/BitPonyLLC/huekeys/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
