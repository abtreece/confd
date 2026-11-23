package main

import (
	"github.com/alecthomas/kong"
)

func main() {
	var cli CLI
	ctx := kong.Parse(&cli, parserOptions()...)
	err := ctx.Run(&cli)
	ctx.FatalIfErrorf(err)
}
