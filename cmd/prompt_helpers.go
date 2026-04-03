package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func promptInput(label, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	for {
		if defaultValue != "" {
			fmt.Printf("%s [%s]: ", label, defaultValue)
		} else {
			fmt.Printf("%s: ", label)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("Failed to read input", "error", err.Error())
			os.Exit(1)
		}

		input = strings.TrimSpace(input)
		if input == "" && defaultValue != "" {
			return defaultValue
		}
		if input != "" {
			return input
		}
	}
}

func promptPassword(label, defaultValue string) string {
	for {
		if defaultValue != "" {
			fmt.Printf("%s [press enter to use current default]: ", label)
		} else {
			fmt.Printf("%s: ", label)
		}

		var input string
		if term.IsTerminal(int(os.Stdin.Fd())) {
			raw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				slog.Error("Failed to read password", "error", err.Error())
				os.Exit(1)
			}
			input = strings.TrimSpace(string(raw))
		} else {
			reader := bufio.NewReader(os.Stdin)
			raw, err := reader.ReadString('\n')
			if err != nil {
				slog.Error("Failed to read password", "error", err.Error())
				os.Exit(1)
			}
			input = strings.TrimSpace(raw)
		}

		if input == "" && defaultValue != "" {
			return defaultValue
		}
		if input != "" {
			return input
		}
	}
}

func promptYesNo(label string, defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	defaultLabel := "y/N"
	if defaultYes {
		defaultLabel = "Y/n"
	}

	for {
		fmt.Printf("%s [%s]: ", label, defaultLabel)
		input, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("Failed to read input", "error", err.Error())
			os.Exit(1)
		}

		switch strings.ToLower(strings.TrimSpace(input)) {
		case "":
			return defaultYes
		case "y", "yes":
			return true
		case "n", "no":
			return false
		}
	}
}

func promptForMissingAppConfig(cmd *cobra.Command, dbname, username, password string) (string, string, string) {
	if !cmd.Flags().Changed("db-name") {
		dbname = promptInput("Database name", dbname)
	}
	if !cmd.Flags().Changed("db-user") {
		username = promptInput("Database user", username)
	}
	if !cmd.Flags().Changed("db-password") {
		password = promptPassword("Database password", password)
	}

	return dbname, username, password
}
