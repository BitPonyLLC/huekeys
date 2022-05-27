package main

import (
	"os"

	"github.com/BitPonyLLC/huekeys/cmd"
)

func main() {
	// sudoUID := os.Getenv("SUDO_UID")
	// uid, err := strconv.Atoi(sudoUID)
	// if err != nil {
	// 	panic(err)
	// }

	// err = syscall.Setuid(uid)
	// if err != nil {
	// 	panic(err)
	// }

	// println("running as original user!")
	os.Exit(cmd.Execute())
}
