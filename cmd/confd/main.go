package main

import (
	"github.com/alecthomas/kong"
)

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("confd"),
		kong.Description("Manage local config files using templates and data from backends"),
		kong.UsageOnError(),
	)
	err := ctx.Run(&cli)
	ctx.FatalIfErrorf(err)
}
