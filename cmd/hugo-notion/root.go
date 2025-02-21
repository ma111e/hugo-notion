package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile         string
	contentDir      string
	notionURL       string
	withFrontMatter bool
	interactive     bool
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./.hugo-notion.yml)")
	rootCmd.PersistentFlags().StringVarP(&contentDir, "content-dir", "d", "", "content directory (default is ./content)")
	rootCmd.PersistentFlags().StringVarP(&notionURL, "url", "u", "", "Notion page URL to sync")
	rootCmd.PersistentFlags().BoolVarP(&withFrontMatter, "front-matter", "f", false, "add front matter in markdown files")
	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "enable interactive page selection")
	// Mark url flag as required
	rootCmd.MarkPersistentFlagRequired("url")

	viper.BindPFlag("content_dir", rootCmd.PersistentFlags().Lookup("content-dir"))
	viper.BindPFlag("notion_url", rootCmd.PersistentFlags().Lookup("url"))
	viper.BindPFlag("front_matter", rootCmd.PersistentFlags().Lookup("front-matter"))
	viper.BindPFlag("interactive", rootCmd.PersistentFlags().Lookup("interactive"))
}

var rootCmd = &cobra.Command{
	Use:   "hugo-notion",
	Short: "Sync Notion pages to markdown files",
	Long: `A CLI tool to synchronize Notion pages and databases to markdown files.
It supports continuous syncing and provides a nice TUI to show sync progress.`,
	RunE: runSync,
}

func Execute() error {
	return rootCmd.Execute()
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yml")
		viper.SetConfigName(".hugo-notion")
	}

	viper.SetEnvPrefix("NOTION")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	// Set defaults
	if viper.GetString("content_dir") == "" {
		absPath, _ := filepath.Abs("./content")
		viper.SetDefault("content_dir", absPath)
	}
}
