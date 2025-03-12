package cmd

import (
	"path/filepath"
	"strings"
)

func fileNameWithoutExtension(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
} /*

// llm junk for reference while on addFullPathToConfig
func loadMultipleConfigs(configPaths []string, configNames []string) error {
	viper.Reset() // Reset viper to avoid conflicts with previous configurations
	for _, path := range configPaths {
		for _, name := range configNames {
			viper.AddConfigPath(path)
			viper.SetConfigName(name)
			viper.SetConfigType(filepath.Ext(name)[1:]) // Extract extension and set config type

			err := viper.ReadInConfig()
			if err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); ok {
					fmt.Printf("Config file %s not found, skipping\n", name)
				} else {
					return fmt.Errorf("error reading config file %s: %w", name, err)
				}
			} else {
				// Merge the config instead of setting it directly
				if err := viper.MergeInConfig(); err != nil {
					return fmt.Errorf("error merging config file %s: %w", name, err)
				}
				fmt.Printf("Config file %s loaded and merged\n", name)
			}
		}
	}
	return nil
}


func testConfigMerge() {
	configPaths := []string{"./config", "/etc/app"}
	configNames := []string{"base.yaml", "override.yaml", "local.yaml"}

	if err := loadMultipleConfigs(configPaths, configNames); err != nil {
		fmt.Println("Error loading configurations:", err)
		return
	}

	fmt.Println("Configuration loaded successfully:")
	fmt.Println("Server Port:", viper.GetString("server.port"))
	fmt.Println("Database URL:", viper.GetString("database.url"))
}
*/
