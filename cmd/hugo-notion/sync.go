package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jomei/notionapi"
	"github.com/ma111e/hugo-notion/internal/sync"
	"github.com/ma111e/hugo-notion/internal/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"net/url"
	"strings"
)

func runSync(_ *cobra.Command, _ []string) error {
	var selectedPages []string

	notionToken := viper.GetString("notion_token")
	if notionToken == "" {
		return fmt.Errorf("Notion token not provided")
	}

	contentNotionUrl := viper.GetString("notion_url")
	if contentNotionUrl == "" {
		return fmt.Errorf("Notion URL not provided")
	}

	pageID, err := extractPageID(contentNotionUrl)
	if err != nil {
		return fmt.Errorf("failed to extract page ID: %v", err)
	}

	client := notionapi.NewClient(notionapi.Token(notionToken))

	isInteractive := viper.GetBool("interactive")

	if isInteractive {
		// Run selection UI
		selectionModel := tui.NewSelectionModel(client, pageID)
		p := tea.NewProgram(selectionModel, tea.WithAltScreen())

		m, err := p.Run()
		if err != nil {
			return fmt.Errorf("error running selection UI: %v", err)
		}

		selModel, ok := m.(tui.SelectionModel)
		if !ok {
			return fmt.Errorf("unexpected model type")
		}

		selectedPages = selModel.GetSelectedPages()
		if len(selectedPages) == 0 {
			fmt.Println("No pages selected, exiting...")
			return nil
		}

	}
	updates := make(chan sync.SyncResult)
	syncer := sync.NewSyncerWithSelection(client, viper.GetString("content_dir"), selectedPages, updates)

	p := tea.NewProgram(tui.NewSyncModel())
	go func() {
		go func() {
			for update := range updates {
				p.Send(update)
			}
		}()

		results := syncer.Sync(pageID)
		close(updates)
		p.Send(results)
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %v", err)
	}
	//}

	return nil
}

func extractPageID(urlStr string) (string, error) {
	parsedURL, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	pathFragments := strings.Split(parsedURL.Path, "-")
	return pathFragments[len(pathFragments)-1], nil
}
