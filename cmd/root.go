/*
Copyright © 2025 Justin Trahan <justin@trahan.dev>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/babbage88/infra-cli/internal/pretty"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	sshKeyPath                              string
	sshKeyPass                              string
	sshRemoteTargetHost                     string
	sshRemoteTargetUser                     string
	cpuProfilePath                          string
	sshUseAgent                             bool
	sshPort                                 uint
	jwtAuthToken                            string
	cfgFile, metaCfgFile, dnsCfgFile        string
	apiTokens                               map[string]string
	rootDomainName                          string
	rawFlag                                 bool
	suplementalCfg                          []string
	rootViperCfg, dnsViperCfg, metaViperCfg *viper.Viper
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
		"Config file (default is default.yaml)")

	rootCmd.PersistentFlags().BoolVarP(&rawFlag, "raw", "r", false,
		"Display command output without any color or highlighting.")

	rootCmd.PersistentFlags().StringToStringVar(&apiTokens, "api-tokens", nil,
		"A string map to store API tokens use provider name as key. eg: api-tokens coudflare='token123'")

	rootCmd.PersistentFlags().StringVar(&jwtAuthToken, "auth-token", "",
		"JWT Token for authentication with both manager or external WebAPIs")

	rootCmd.PersistentFlags().StringVar(&rootDomainName, "domain-name", "",
		"The root domain/zone name for which dns changes or queries will be made. ")

	rootCmd.PersistentFlags().StringVar(&cpuProfilePath, "cpu-profile", "",
		"File path to create cpu profile for preformance testing.")

	rootCmd.PersistentFlags().StringArrayVarP(&suplementalCfg, "optional-config", "k", nil, "Additional config viles to merge.")

	rootCmd.PersistentFlags().StringVar(&sshKeyPath, "ssh-key", "",
		"Path to SSH Key for performing tasks on remote hosts")

	rootCmd.PersistentFlags().StringVar(&sshKeyPass, "ssh-passphrase", "",
		"Passphrase for ssh-key")

	rootCmd.PersistentFlags().StringVar(&sshRemoteTargetHost, "ssh-remote-host", "",
		"The remote hostname or IP to run remote functions")

	rootCmd.PersistentFlags().StringVar(&sshRemoteTargetUser, "ssh-remote-user", "",
		"Username for remote host connection")

	rootCmd.PersistentFlags().BoolVar(&sshUseAgent, "ssh-use-agent", false,
		"Use ssh-agent for ssh-key auth.")

	rootCmd.PersistentFlags().UintVar(&sshPort, "ssh-port", uint(22),
		"Port SSH is listening on the remote host")

	// Read Viper config before execution
	cobra.OnInitialize(func() {
		initConfig()
	})
}

func initConfig() {
	err := loadRootConfigFile()
	if err != nil {
		pretty.PrintErrorf("error loading root config %s", err.Error())
	}

	rootViperCfg.SetDefault("api_tokens", apiTokens)
	rootViperCfg.BindPFlag("api_tokens", rootCmd.PersistentFlags().Lookup("api-tokens"))
	rootViperCfg.BindPFlag("auth_token", rootCmd.PersistentFlags().Lookup("auth-token"))
	rootViperCfg.BindPFlag("domain_name", rootCmd.PersistentFlags().Lookup("domain-name"))
	rootViperCfg.BindPFlag("ssh_key", rootCmd.PersistentFlags().Lookup("ssh-key"))
	rootViperCfg.BindPFlag("ssh_passphrase", rootCmd.PersistentFlags().Lookup("ssh-passphrase"))
	rootViperCfg.BindPFlag("ssh_use_agent", rootCmd.PersistentFlags().Lookup("ssh-use-agent"))
	rootViperCfg.BindPFlag("ssh_port", rootCmd.PersistentFlags().Lookup("ssh-port"))
	rootViperCfg.BindPFlag("ssh_remote_host", rootCmd.PersistentFlags().Lookup("ssh-remote-host"))
	rootViperCfg.BindPFlag("ssh_remote_user", rootCmd.PersistentFlags().Lookup("ssh-remote-user"))
	rootViperCfg.BindPFlag("optional_config", rootCmd.PersistentFlags().Lookup("optional-config"))
	rootViperCfg.BindPFlag("cpu_profile", rootCmd.PersistentFlags().Lookup("cpu-profile"))

	rootViperCfg.AutomaticEnv()

	apiTokens = rootViperCfg.GetStringMapString("api_tokens")

	if jwtAuthToken == "" {
		jwtAuthToken = rootViperCfg.GetString("auth_token")
	}

	jwtKeyName = rootViperCfg.GetString("jwt_key_name")
	jwtTokenName = rootViperCfg.GetString("jwt_token_name")
	jwtAuthToken = rootViperCfg.GetString("jwt_secret")
}

func GetConfigPath() string {
	dirname, err := os.UserHomeDir()
	if err != nil {
		pretty.PrintErrorf("error retrieving the current user's home dir. error: %s", err.Error())
	}
	infractldotConfigDir := filepath.Join(dirname, ".config", "infractl")
	return infractldotConfigDir
}

func loadRootConfigFile() error {
	rootViperCfg = viper.New() // Use viper.New() instead of &viper.Viper{}
	infractldotConfigDir := GetConfigPath()

	if cfgFile != "" {
		baseFile := fileNameWithoutExtension(cfgFile)
		basecfgDir := filepath.Dir(cfgFile)
		cfgExtension := filepath.Ext(cfgFile)[1:]

		rootViperCfg.SetConfigName(baseFile)
		rootViperCfg.SetConfigType(cfgExtension)
		rootViperCfg.AddConfigPath(basecfgDir)
		rootViperCfg.SetConfigFile(cfgFile)
	} else {
		rootViperCfg.SetConfigName("default")
		rootViperCfg.SetConfigType("yaml")
		rootViperCfg.AddConfigPath(".")
		rootViperCfg.AddConfigPath(infractldotConfigDir)
		rootViperCfg.AddConfigPath(".config/infractl")
	}

	if err := rootViperCfg.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read root config: %w", err)
	}

	rootViperCfg.WatchConfig()
	return nil
}

func mergeMetaConfigFile() error {
	infractldotConfigDir := GetConfigPath()
	if metaCfgFile != "" {
		metaViperCfg = viper.New()
		baseFile := fileNameWithoutExtension(metaCfgFile)
		basecfgDir := filepath.Dir(metaCfgFile)
		cfgExtesnion := filepath.Ext(metaCfgFile)[1:]
		metaViperCfg.SetConfigName(baseFile)
		metaViperCfg.SetConfigType(cfgExtesnion)
		metaViperCfg.AddConfigPath(basecfgDir)
		metaViperCfg.AddConfigPath(infractldotConfigDir)
		if err := metaViperCfg.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read meta config: %w", err)
		}
		metaViperCfg.WatchConfig()
	}

	return nil
}

func mergeDnsConfigFile() error {
	infractldotConfigDir := GetConfigPath()
	if dnsCfgFile != "" {
		dnsViperCfg = viper.New()
		baseFile := fileNameWithoutExtension(dnsCfgFile)
		basecfgDir := filepath.Dir(dnsCfgFile)
		cfgExtesnion := filepath.Ext(dnsCfgFile)[1:]
		dnsViperCfg.SetConfigName(baseFile)
		dnsViperCfg.SetConfigType(cfgExtesnion)
		dnsViperCfg.AddConfigPath(basecfgDir)
		dnsViperCfg.AddConfigPath(infractldotConfigDir)
		if err := dnsViperCfg.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read dns config: %w", err)
		}
		dnsViperCfg.WatchConfig()
	}

	return nil
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
