package sync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/ma111e/notion2markdown"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jomei/notionapi"
	"github.com/samber/lo"
	"gopkg.in/yaml.v2"
)

// Regular expression to find Markdown image tags
var imageRegex = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)

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

// downloadImage downloads an image from a URL and saves it to the specified path
func (s *Syncer) downloadImage(imageURL, destPath string) error {
	resp, err := http.Get(imageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// generateImageFilename generates a unique filename for an image based on its URL
func generateImageFilename(imageURL string) string {
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		// If URL parsing fails, use a hash of the full URL
		hash := sha256.Sum256([]byte(imageURL))
		return fmt.Sprintf("%x%s", hash[:8], filepath.Ext(imageURL))
	}

	// Use the last part of the path as the filename
	basename := filepath.Base(parsedURL.Path)
	if basename == "" || basename == "." {
		// If no filename in URL, use a hash
		hash := sha256.Sum256([]byte(imageURL))
		return fmt.Sprintf("%x%s", hash[:8], filepath.Ext(imageURL))
	}

	return basename
}

// processImages processes all images in the markdown content
func (s *Syncer) processImages(markdown string, postDir string, sanitizedName string) string {
	if viper.GetBool("s3_images") {
		return markdown // Return unchanged if using S3
	}

	baseURI := strings.TrimRight(viper.GetString("posts_base_uri"), "/")

	return imageRegex.ReplaceAllStringFunc(markdown, func(match string) string {
		submatches := imageRegex.FindStringSubmatch(match)
		if len(submatches) != 3 {
			return match
		}

		rawAlt := submatches[1]
		imageURL := submatches[2]

		chunks := strings.SplitN(rawAlt, "|alt:", 2)
		caption := chunks[0]
		alt := chunks[1]

		// Generate filename and paths
		filename := generateImageFilename(imageURL)
		imagePath := filepath.Join(postDir, "images", filename)
		relativeImagePath := fmt.Sprintf("%s/%s/images/%s", baseURI, sanitizedName, filename)

		// Download the image
		if err := s.downloadImage(imageURL, imagePath); err != nil {
			// If download fails, return original markdown
			return match
		}

		// Return Hugo shortcode
		return fmt.Sprintf("\n\n{{< figure src=\"%s\" caption=\"%s\" alt=\"%s\" position=\"center\" captionStyle=\"font-style: italic;\" >}}\n\n",
			relativeImagePath,
			caption,
			alt)
	})
}

func (s *Syncer) syncChildPage(block *notionapi.ChildPageBlock, hugoPageDir string, syncTime time.Time, syncedHugoPageFilePaths *[]string) {
	childPageId := block.GetID()
	childPageTitle := block.ChildPage.Title
	childPageLastEditedAt := *block.GetLastEditedTime()

	if len(s.selectedPages) > 0 && !slices.Contains(s.selectedPages, string(childPageId)) {
		return
	}

	sanitizedName := strings.ReplaceAll(strings.ToLower(childPageTitle), " ", "_")
	postDir := filepath.Join(hugoPageDir, sanitizedName)
	imagesDir := filepath.Join(postDir, "images")

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

	hugoPageFileName := sanitizedName + ".md"
	hugoPageFilePath := filepath.Join(postDir, hugoPageFileName)
	*syncedHugoPageFilePaths = append(*syncedHugoPageFilePaths, hugoPageFilePath)

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

	// Process images in the markdown content
	markdown = s.processImages(markdown, postDir, sanitizedName)

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
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Updated",
			Path:        hugoPageFilePath,
			LastUpdated: syncTime,
		})
	} else {
		s.addResult(SyncResult{
			PageTitle:   childPageTitle,
			Status:      "Created",
			Path:        hugoPageFilePath,
			LastUpdated: syncTime,
		})
	}

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
