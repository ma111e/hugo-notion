package main

import (
	"fmt"
	_ "github.com/joho/godotenv/autoload"
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
	useS3Images     bool
	postsBaseURI    string
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./.hugo-notion.yml)")
	rootCmd.PersistentFlags().StringVarP(&contentDir, "content-dir", "d", "./content/posts", "content directory (default is ./content/posts)")
	rootCmd.PersistentFlags().StringVarP(&notionURL, "url", "u", "", "Notion page URL to sync")
	rootCmd.PersistentFlags().StringVarP(&notionToken, "token", "t", "", "Notion token of the integration connected to the root page to fetch")
	rootCmd.PersistentFlags().BoolVarP(&withFrontMatter, "add-front-matter", "a", false, "add front matter in markdown files")
	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "enable interactive page selection")
	rootCmd.PersistentFlags().BoolVar(&useS3Images, "s3-images", false, "use S3 for image storage (legacy behavior)")
	rootCmd.PersistentFlags().StringVar(&postsBaseURI, "posts-base-uri", "/posts", "base URI for posts in the generated site")

	viper.BindPFlag("content_dir", rootCmd.PersistentFlags().Lookup("content-dir"))
	viper.BindPFlag("notion_token", rootCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("notion_root_page", rootCmd.PersistentFlags().Lookup("url"))
	viper.BindPFlag("add_front_matter", rootCmd.PersistentFlags().Lookup("front-matter"))
	viper.BindPFlag("interactive", rootCmd.PersistentFlags().Lookup("interactive"))
	viper.BindPFlag("s3_images", rootCmd.PersistentFlags().Lookup("s3-images"))
	viper.BindPFlag("posts_base_uri", rootCmd.PersistentFlags().Lookup("posts-base-uri"))
}

var rootCmd = &cobra.Command{
	Use:   "hugo-notion",
	Short: "Sync Notion pages to markdown files",
	Long:  `A CLI tool to synchronize Notion pages and databases to markdown files`,
	PreRun: func(cmd *cobra.Command, args []string) {
		// godotenv's autoload is used
		viper.SetEnvPrefix("HN")
		viper.SetConfigFile(".hugo-notion.yml")
		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err == nil {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		} else {
			fmt.Println(err)
		}
	},
	RunE: runSync,
}

func Execute() error {
	return rootCmd.Execute()
}
