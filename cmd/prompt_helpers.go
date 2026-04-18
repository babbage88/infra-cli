package cmd

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
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

func promptInputWithExample(label, example, defaultValue string) string {
	promptLabel := label
	if strings.TrimSpace(example) != "" {
		promptLabel = fmt.Sprintf("%s (example: %s)", label, example)
	}
	return promptInput(promptLabel, defaultValue)
}

func promptOptionalInput(label, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

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
	if input == "" {
		return defaultValue
	}

	return input
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
			raw, err := readMaskedInput(int(os.Stdin.Fd()))
			if err != nil {
				slog.Error("Failed to read password", "error", err.Error())
				os.Exit(1)
			}
			input = strings.TrimSpace(raw)
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

func readMaskedInput(fd int) (string, error) {
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, oldState)

	var builder strings.Builder
	buffer := make([]byte, 1)

	for {
		n, err := os.Stdin.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println()
				return builder.String(), nil
			}
			return "", err
		}
		if n == 0 {
			continue
		}

		switch buffer[0] {
		case '\r', '\n':
			fmt.Println()
			return builder.String(), nil
		case 3:
			fmt.Println()
			return "", io.EOF
		case 127, 8:
			if builder.Len() == 0 {
				continue
			}
			current := builder.String()
			builder.Reset()
			builder.WriteString(current[:len(current)-1])
			fmt.Print("\b \b")
		default:
			builder.WriteByte(buffer[0])
			fmt.Print("*")
		}
	}
}

func promptPasswordWithExample(label, example, defaultValue string) string {
	promptLabel := label
	if strings.TrimSpace(example) != "" {
		promptLabel = fmt.Sprintf("%s (example: %s)", label, example)
	}
	return promptPassword(promptLabel, defaultValue)
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

func promptSelectOption(label string, options []string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)

	if len(options) == 0 {
		if defaultValue != "" {
			return defaultValue
		}
		slog.Error("promptSelectOption called with no options")
		os.Exit(1)
	}

	defaultIndex := -1
	for i, option := range options {
		fmt.Printf("%d. %s\n", i+1, option)
		if defaultValue != "" && option == defaultValue {
			defaultIndex = i
		}
	}

	for {
		switch {
		case defaultIndex >= 0:
			fmt.Printf("%s [%d]: ", label, defaultIndex+1)
		case defaultValue != "":
			fmt.Printf("%s [%s]: ", label, defaultValue)
		default:
			fmt.Printf("%s: ", label)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("Failed to read input", "error", err.Error())
			os.Exit(1)
		}

		input = strings.TrimSpace(input)
		if input == "" {
			if defaultIndex >= 0 {
				return options[defaultIndex]
			}
			if defaultValue != "" {
				return defaultValue
			}
			continue
		}

		selection, err := strconv.Atoi(input)
		if err == nil && selection >= 1 && selection <= len(options) {
			return options[selection-1]
		}

		for _, option := range options {
			if input == option {
				return option
			}
		}

		fmt.Printf("Please enter a number between 1 and %d or a listed value.\n", len(options))
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
