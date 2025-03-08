package main

import (
	"fmt"
	"os"

	"github.com/babbage88/infra-cli/cmd"
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/viper"
)

var logger = pretty.NewCustomLogger(os.Stdout, "DEBUG", 1, "|", true)

func main() {
	// Initialize Viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}

	cmd.Execute()
}
