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

	// Clean up old files
	oldHugoPageFilePaths, _ := lo.Difference(existingHugoPageFilePaths, syncedHugoPageFilePaths)

	// Only delete files in full sync mode
	if !(len(s.selectedPages) > 0) {
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

	hugoPageFileName := strings.ReplaceAll(childPageTitle, " ", "-") + ".md"
	err := os.MkdirAll(hugoPageDir, 0755)
	if err != nil {
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Error",
			Path:        hugoPageDir,
			LastUpdated: time.Now(),
		})
		return
	}

	hugoPageFilePath := filepath.Join(hugoPageDir, hugoPageFileName)
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

//func (s *Syncer) syncChildDatabase(block notionapi.Block, destinationDir string, syncTime time.Time, syncedHugoPageFilePaths *[]string) {
//	dbBlock := block.(*notionapi.ChildDatabaseBlock)
//	dbID := notionapi.DatabaseID(dbBlock.GetID())
//	dbTitle := dbBlock.ChildDatabase.Title
//	hugoPageDir := filepath.Join(destinationDir, dbTitle)
//
//	existingHugoPageFilePaths, err := filepath.Glob(filepath.Join(hugoPageDir, "*.md"))
//	if err != nil {
//		s.addResult(SyncResult{
//			PageTitle:   dbTitle,
//			Status:      "Error",
//			Path:        hugoPageDir,
//			LastUpdated: time.Now(),
//		})
//		return
//	}
//
//	// Query database pages
//	databaseQueryRequest := notionapi.DatabaseQueryRequest{
//		PageSize: 100,
//	}
//
//	databaseQueryResponse, err := s.client.Database.Query(context.Background(), dbID, &databaseQueryRequest)
//	if err != nil {
//		s.addResult(SyncResult{
//			PageTitle:   dbTitle,
//			Status:      "Error",
//			Path:        hugoPageDir,
//			LastUpdated: time.Now(),
//		})
//		return
//	}
//
//	// Process each database page
//	for _, page := range databaseQueryResponse.Results {
//		titleProp := page.Properties["Name"].(*notionapi.TitleProperty)
//		childPageTitle := titleProp.Title[0].PlainText
//		childPageLastEditedAt := page.LastEditedTime
//
//		hugoPageFileName := strings.ReplaceAll(childPageTitle, " ", "-") + ".md"
//		err = os.MkdirAll(hugoPageDir, 0755)
//		if err != nil {
//			s.addResult(SyncResult{
//				PageTitle:   childPageTitle,
//				Status:      "Error",
//				Path:        hugoPageDir,
//				LastUpdated: time.Now(),
//			})
//			continue
//		}
//
//		hugoPageFilePath := filepath.Join(hugoPageDir, hugoPageFileName)
//		*syncedHugoPageFilePaths = append(*syncedHugoPageFilePaths, hugoPageFilePath)
//
//		if doesFileExist(hugoPageFilePath) && !isFileModTimeRoundedToNearestMinuteLessThanOrEqualTo(hugoPageFilePath, childPageLastEditedAt) {
//			s.addResult(SyncResult{
//				PageTitle:   childPageTitle,
//				Status:      "Skipped",
//				Path:        hugoPageFilePath,
//				LastUpdated: childPageLastEditedAt,
//			})
//			continue
//		}
//
//		// Create front matter
//		frontMatter := map[string]string{
//			"title": childPageTitle,
//			"date":  page.Properties["date"].(*notionapi.DateProperty).Date.Start.String(),
//		}
//
//		frontMatterYaml, err := yaml.Marshal(frontMatter)
//		if err != nil {
//			s.addResult(SyncResult{
//				PageTitle:   childPageTitle,
//				Status:      "Error",
//				Path:        hugoPageFilePath,
//				LastUpdated: time.Now(),
//			})
//			continue
//		}
//
//		// Convert to markdown
//		markdown, err := notion2markdown.PageToMarkdown(s.client, string(page.ID))
//		if err != nil {
//			s.addResult(SyncResult{
//				PageTitle:   childPageTitle,
//				Status:      "Error",
//				Path:        hugoPageFilePath,
//				LastUpdated: time.Now(),
//			})
//			continue
//		}
//
//		// Write the file
//		pageText := fmt.Sprintf("---\n%s\n---\n\n%s", frontMatterYaml, markdown)
//		err = os.WriteFile(hugoPageFilePath, []byte(pageText), 0644)
//		if err != nil {
//			s.addResult(SyncResult{
//				PageTitle:   childPageTitle,
//				Status:      "Error",
//				Path:        hugoPageFilePath,
//				LastUpdated: time.Now(),
//			})
//			continue
//		}
//
//		os.Chtimes(hugoPageFilePath, syncTime, syncTime)
//		s.addResult(SyncResult{
//			PageTitle:   childPageTitle,
//			Status:      "Updated",
//			Path:        hugoPageFilePath,
//			LastUpdated: syncTime,
//		})
//	}
//
//	// Clean up old files
//	oldHugoPageFilePaths, _ := lo.Difference(existingHugoPageFilePaths, *syncedHugoPageFilePaths)
//	s.deleteFiles(oldHugoPageFilePaths)
//}

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

// Helper functions
func doesFileExist(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

//func isFileModTimeRoundedToNearestMinuteLessThanOrEqualTo(filePath string, _time time.Time) bool {
//	fileInfo, err := os.Stat(filePath)
//	if err != nil {
//		return true // If we can't get file info, assume we should update
//	}
//	fileModTimeRoundedToNearestMinute := fileInfo.ModTime().Truncate(time.Minute)
//	return _time.After(fileModTimeRoundedToNearestMinute)
//}
