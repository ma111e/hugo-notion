package sync

import (
	"bytes"
	"context"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jomei/notionapi"
	"github.com/nisanthchunduru/notion2markdown"
	"github.com/samber/lo"
	"gopkg.in/yaml.v2"
)

type SyncResult struct {
	PageTitle   string
	Status      string
	Path        string
	LastUpdated time.Time
}

type Syncer struct {
	client        *notionapi.Client
	contentDir    string
	results       []SyncResult
	selectedPages []string
	updates       chan<- SyncResult // Channel for live updates
}

func NewSyncer(client *notionapi.Client, contentDir string) *Syncer {
	return &Syncer{
		client:     client,
		contentDir: contentDir,
		results:    make([]SyncResult, 0),
	}
}

func NewSyncerWithSelection(client *notionapi.Client, contentDir string, selectedPages []string, updates chan<- SyncResult) *Syncer {
	return &Syncer{
		client:        client,
		contentDir:    contentDir,
		results:       make([]SyncResult, 0),
		selectedPages: selectedPages,
		updates:       updates,
	}
}

func (s *Syncer) Sync(pageID string) []SyncResult {
	s.results = make([]SyncResult, 0)
	s.syncPage(pageID, s.contentDir)
	return s.results
}

func (s *Syncer) syncPage(pageIDString string, destinationDir string) {
	pageID := notionapi.BlockID(pageIDString)
	pagination := notionapi.Pagination{
		PageSize: 100,
	}

	getChildrenResponse, err := s.client.Block.GetChildren(context.Background(), pageID, &pagination)
	if err != nil {
		s.addResult(SyncResult{
			PageTitle:   "Root Page",
			Status:      "Error",
			Path:        destinationDir,
			LastUpdated: time.Now(),
		})
		return
	}

	syncTime := time.Now()
	syncedHugoPageFilePaths := []string{}
	hugoPageDir := destinationDir

	existingHugoPageFilePaths, err := filepath.Glob(filepath.Join(hugoPageDir, "*.md"))
	if err != nil {
		s.addResult(SyncResult{
			PageTitle:   "File Scan",
			Status:      "Error",
			Path:        hugoPageDir,
			LastUpdated: time.Now(),
		})
		return
	}

	for _, _block := range getChildrenResponse.Results {
		blockID := string(_block.GetID())

		// Skip if not selected (when in selective mode)
		if len(s.selectedPages) > 0 && !slices.Contains(s.selectedPages, blockID) {
			continue
		}

		s.syncChildPage(_block.(*notionapi.ChildPageBlock), hugoPageDir, syncTime, &syncedHugoPageFilePaths)
	}

	// Only delete files in full sync mode
	if len(s.selectedPages) == 0 {
		// Clean up old files
		oldHugoPageFilePaths, _ := lo.Difference(existingHugoPageFilePaths, syncedHugoPageFilePaths)

		s.deleteFiles(oldHugoPageFilePaths)
	}
}

func (s *Syncer) syncChildPage(block *notionapi.ChildPageBlock, hugoPageDir string, syncTime time.Time, syncedHugoPageFilePaths *[]string) {
	childPageId := block.GetID()
	childPageTitle := block.ChildPage.Title
	childPageLastEditedAt := *block.GetLastEditedTime()

	// Skip if not selected (when in selective mode)
	if len(s.selectedPages) > 0 && !slices.Contains(s.selectedPages, string(childPageId)) {
		return
	}

	// Create sanitized filename/folder name
	sanitizedName := strings.ReplaceAll(strings.ToLower(childPageTitle), " ", "_")

	// Create the post-specific directory structure
	postDir := filepath.Join(hugoPageDir, "posts", sanitizedName)
	imagesDir := filepath.Join(postDir, "images")

	// Create all necessary directories
	if err := os.MkdirAll(postDir, 0755); err != nil {
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Error",
			Path:        postDir,
			LastUpdated: time.Now(),
		})
		return
	}

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Error",
			Path:        imagesDir,
			LastUpdated: time.Now(),
		})
		return
	}

	// Set markdown filename to match directory name
	hugoPageFileName := sanitizedName + ".md"
	hugoPageFilePath := filepath.Join(postDir, hugoPageFileName)
	*syncedHugoPageFilePaths = append(*syncedHugoPageFilePaths, hugoPageFilePath)

	// Convert to markdown
	markdown, err := notion2markdown.PageToMarkdown(s.client, string(childPageId))
	if err != nil {
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Error",
			Path:        hugoPageFilePath,
			LastUpdated: time.Now(),
		})
		return
	}

	// Generate new content
	var newContent string
	if viper.GetBool("front_matter") {
		hugoPageFrontMatterMap := map[string]string{
			"title": childPageTitle,
			"type":  childPageTitle,
			"date":  childPageLastEditedAt.Format(time.RFC3339),
		}

		hugoFrontMatterYaml, err := yaml.Marshal(hugoPageFrontMatterMap)
		if err != nil {
			s.addResult(SyncResult{
				PageTitle:   childPageTitle,
				Status:      "Error",
				Path:        hugoPageFilePath,
				LastUpdated: time.Now(),
			})
			return
		}
		newContent = fmt.Sprintf("---\n%s\n---\n\n%s", hugoFrontMatterYaml, markdown)
	} else {
		newContent = markdown
	}

	// Check if file exists and compare content
	if existingContent, err := os.ReadFile(hugoPageFilePath); err == nil {
		if bytes.Equal([]byte(newContent), existingContent) {
			s.addResult(SyncResult{
				PageTitle:   childPageTitle,
				Status:      "Skipped",
				Path:        hugoPageFilePath,
				LastUpdated: childPageLastEditedAt,
			})
			return
		}
		// File exists but content is different
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Updated",
			Path:        hugoPageFilePath,
			LastUpdated: syncTime,
		})
	} else {
		// File doesn't exist - this is a new page
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Created",
			Path:        hugoPageFilePath,
			LastUpdated: syncTime,
		})
	}

	// Write the file
	err = os.WriteFile(hugoPageFilePath, []byte(newContent), 0644)
	if err != nil {
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Error",
			Path:        hugoPageFilePath,
			LastUpdated: time.Now(),
		})
		return
	}

	os.Chtimes(hugoPageFilePath, syncTime, syncTime)
}

func (s *Syncer) addResult(result SyncResult) {
	s.results = append(s.results, result)
	if s.updates != nil {
		s.updates <- result
	}
}

func (s *Syncer) deleteFiles(filePaths []string) {
	for _, filePath := range filePaths {
		err := os.Remove(filePath)
		if err != nil {
			s.addResult(SyncResult{
				PageTitle:   filepath.Base(filePath),
				Status:      "Delete Error",
				Path:        filePath,
				LastUpdated: time.Now(),
			})
			continue
		}
		s.addResult(SyncResult{
			PageTitle:   filepath.Base(filePath),
			Status:      "Deleted",
			Path:        filePath,
			LastUpdated: time.Now(),
		})
	}
}
