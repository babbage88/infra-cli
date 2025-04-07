package main

import (
	"embed"

	"github.com/babbage88/infra-cli/cmd"
)

//go:embed remote_utils/*
var remoteUtils embed.FS

func main() {
	cmd.Execute()
}
