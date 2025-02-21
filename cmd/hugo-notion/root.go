package main

import (
	"fmt"
	_ "github.com/joho/godotenv/autoload"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile         string
	contentDir      string
	notionURL       string
	notionToken     string
	withFrontMatter bool
	interactive     bool
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./.hugo-notion.yml)")
	rootCmd.PersistentFlags().StringVarP(&contentDir, "content-dir", "d", "", "content directory (default is ./content)")
	rootCmd.PersistentFlags().StringVarP(&notionURL, "url", "u", "", "Notion page URL to sync")
	rootCmd.PersistentFlags().StringVarP(&notionToken, "token", "t", "", "Notion token of the integration connected to the root page to fetch")
	rootCmd.PersistentFlags().BoolVarP(&withFrontMatter, "front-matter", "f", false, "add front matter in markdown files")
	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "enable interactive page selection")

	viper.BindPFlag("content_dir", rootCmd.PersistentFlags().Lookup("content-dir"))
	viper.BindPFlag("notion_token", rootCmd.PersistentFlags().Lookup("token"))
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
	// godotenv's autoload is used
	viper.SetEnvPrefix("HN")
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
