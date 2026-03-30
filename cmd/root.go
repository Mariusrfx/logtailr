package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	dbURL   string
)

var rootCmd = &cobra.Command{
	Use:     "logtailr",
	Short:   "Concurrent multi-source log aggregator",
	Long:    `Logtailr is a high-performance CLI tool to tail, parse, and filter logs from files, Docker, and journalctl simultaneously.`,
	Version: "v0.1.0",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().StringVar(&dbURL, "db-url", "", "PostgreSQL connection URL (env: LOGTAILR_DB_URL)")
}

func initConfig() {
	if cfgFile != "" {
		absPath, err := filepath.Abs(cfgFile)
		if err != nil {
			fmt.Printf("Error: invalid config file path: %v\n", err)
			os.Exit(1)
		}
		absPath, err = filepath.EvalSymlinks(absPath)
		if err != nil {
			fmt.Printf("Error: cannot resolve config file path: %v\n", err)
			os.Exit(1)
		}
		fi, err := os.Stat(absPath)
		if err != nil {
			fmt.Printf("Error: cannot access config file: %v\n", err)
			os.Exit(1)
		}
		if !fi.Mode().IsRegular() {
			fmt.Printf("Error: config path is not a regular file\n")
			os.Exit(1)
		}
		viper.SetConfigFile(absPath)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("LOGTAILR")

	// Bind --db-url flag and LOGTAILR_DB_URL env var
	_ = viper.BindPFlag("database.url", rootCmd.PersistentFlags().Lookup("db-url"))
	_ = viper.BindEnv("database.url", "LOGTAILR_DB_URL")

	// Bind LOGTAILR_API_TOKEN env var
	_ = viper.BindEnv("api.token", "LOGTAILR_API_TOKEN")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Printf("Error reading config file: %v\n", err)
			os.Exit(1)
		}
	}
}
