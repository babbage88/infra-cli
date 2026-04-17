package main

import (
	"log/slog"

	"github.com/babbage88/infra-cli/cmd"
)

func main() {
	configureDefaultLogger(slog.LevelInfo)
	cmd.Execute()
}
