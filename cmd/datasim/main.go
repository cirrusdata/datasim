package main

import (
	"fmt"
	"os"

	"github.com/cirrusdata/datasim/internal/app"
	"github.com/cirrusdata/datasim/internal/cli"
)

var (
	version    = "dev"
	commit     = "none"
	date       = "unknown"
	repository = "cirrusdata/datasim"
)

// main wires the bootstrap and executes the root command.
func main() {
	build := app.BuildInfo{
		Version:    version,
		Commit:     commit,
		Date:       date,
		Repository: repository,
	}

	bootstrap, err := app.New(build)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	root := cli.NewRootCmd(bootstrap)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
