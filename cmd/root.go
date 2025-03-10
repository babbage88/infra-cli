package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	jwtAuthToken   string
	cfgFile        string
	apiTokens      map[string]string
	rootDomainName string
	rawFlag        bool
)

var rootCmd = &cobra.Command{
	Use:   "infractl",
	Short: "Infractl - Manage the universe for you applications",
	Long:  `Commands and utilities for managing a go-infra instance or it's child applications.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Load default config values
	apiTokens = make(map[string]string)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"Config file (default is config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&rawFlag, "raw", "r", false, "Display command output without any color or highlighting.")
	rootCmd.PersistentFlags().StringVar(&cfDnsToken, "cf-token", "",
		"Cloudflare API Token - debugging")
	rootCmd.PersistentFlags().StringToStringVar(&apiTokens, "api-tokens", nil,
		"A string map to store API tokens use provider name as key. eg: api-tokens coudflare='token123'")
	rootCmd.PersistentFlags().StringVar(&jwtAuthToken, "auth-token", "",
		"JWT Token for authentication with both manager or external WebAPIs")
	rootCmd.PersistentFlags().StringVar(&rootDomainName, "domain-name", "",
		"The root domain/zone name for which dns changes or queries will be made. ")

	// Read Viper config before execution
	cobra.OnInitialize(func() {
		initConfig()
	})
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	if err := viper.ReadInConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config file: %v\n", err)
	}

	viper.SetDefault("api_tokens", viper.GetStringMapString("api_tokens"))
	viper.BindPFlag("api_tokens", cloudflareCmd.PersistentFlags().Lookup("api-token"))
	viper.SetDefault("auth_token", viper.GetString("auth_token"))
	viper.BindPFlag("auth_token", cloudflareCmd.PersistentFlags().Lookup("auth-token"))
	viper.SetDefault("domain_name", viper.GetString("domain_name"))
	viper.BindPFlag("domain_name", cloudflareCmd.PersistentFlags().Lookup("domain-name"))
	viper.SetDefault("cf_token", viper.GetString("cf_token"))
	viper.BindPFlag("cf_token", cloudflareCmd.PersistentFlags().Lookup("cf-token"))
	viper.AutomaticEnv()

	apiTokens = viper.GetStringMapString("api_tokens")

	if jwtAuthToken == "" {
		jwtAuthToken = viper.GetString("auth_token")
	}

	jwtKeyName = viper.GetString("jwt_key_name")
	jwtTokenName = viper.GetString("jwt_token_name")
	jwtAuthToken = viper.GetString("jwt_secret")
}

func writeToEnvFile(filename, key, value string) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error writing to .env file:", err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	if err != nil {
		fmt.Println("Error writing to .env file:", err)
	}
}

// writeToYAML writes a key-value pair to a YAML file.
func writeToYAML(filename, key, value string) {
	data := make(map[string]string)

	// Read existing file if it exists
	if file, err := os.ReadFile(filename); err == nil {
		yaml.Unmarshal(file, &data)
	}

	// Update or add the key-value pair
	data[key] = value

	// Write back to file
	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error writing to YAML file:", err)
		return
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	if err := encoder.Encode(data); err != nil {
		fmt.Println("Error encoding YAML:", err)
	}
}

// writeToJSON writes a key-value pair to a JSON file.
func writeToJSON(filename, key, value string) {
	data := make(map[string]string)

	// Read existing file if it exists
	if file, err := os.ReadFile(filename); err == nil {
		json.Unmarshal(file, &data)
	}

	// Update or add the key-value pair
	data[key] = value

	// Write back to file
	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error writing to JSON file:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty-print JSON

	if err := encoder.Encode(data); err != nil {
		fmt.Println("Error encoding JSON:", err)
	}
}

// writeToTOML writes a key-value pair to a TOML file.
func writeToTOML(filename, key, value string) {
	data := make(map[string]string)

	// Read existing file if it exists
	if file, err := os.ReadFile(filename); err == nil {
		toml.Unmarshal(file, &data)
	}

	// Update or add the key-value pair
	data[key] = value

	// Write back to file
	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error writing to TOML file:", err)
		return
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		fmt.Println("Error encoding TOML:", err)
	}
}

func outputToFile(key, value string) {
	if envFile != "" {
		pretty.Printf("Creating file: %s", envFile)
		writeToEnvFile(envFile, key, value)
	}

	if yamlFile != "" {
		pretty.Printf("Creating file: %s", yamlFile)
		writeToYAML(yamlFile, key, value)
	}

	if jsonFile != "" {
		pretty.Printf("Creating file: %s", jsonFile)
		writeToJSON(jsonFile, key, value)
	}

	if tomlFile != "" {
		pretty.Printf("Creating file: %s", tomlFile)
		writeToTOML(tomlFile, key, value)
	}
}
