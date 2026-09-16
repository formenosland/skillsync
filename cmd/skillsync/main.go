package main

import (
	"os"

	"github.com/formenosland/skillsync/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args, os.Stdin, os.Stdout, os.Stderr))
}
