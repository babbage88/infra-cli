package main

import (
	"embed"
	"log/slog"

	"github.com/babbage88/infra-cli/cmd"
)

//go:embed remote_utils/bin/*
var remoteUtils embed.FS

func main() {
	configureDefaultLogger(slog.LevelInfo)
	cmd.Execute()
}
