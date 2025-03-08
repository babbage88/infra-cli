package main

import (
	"fmt"
	"os"

	"github.com/babbage88/infra-cli/cmd"
	"github.com/spf13/viper"
)

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
