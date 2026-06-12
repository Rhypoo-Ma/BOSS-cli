package boss

import (
	"github.com/Rhypoo-Ma/BOSS-cli/browser"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

func DownloadResume(client *browser.Client, candidateName string, downloadDir string) error {
	if downloadDir == "" {
		downloadDir = "."
	}

	// Step 1: click the candidate row
	code := fmt.Sprintf(`(function(){
		var items = document.querySelectorAll('.geek-item-wrap, .geek-item, .friend-item, .chat-item');
		for (var i = 0; i < items.length; i++) {
			if (items[i].textContent.indexOf('%s') > -1) {
				items[i].click();
				return JSON.stringify({success: true});
			}
		}
		return JSON.stringify({success: false, reason: 'candidate not found'});
	})()`, strings.ReplaceAll(candidateName, "'", "\\'"))

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return fmt.Errorf("click candidate failed: %w", err)
	}
	var result struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("open candidate failed: %s", result.Reason)
	}

	time.Sleep(1500 * time.Millisecond)

	// Step 2: look for "click preview" button and click it
	clickPreviewCode := `(function(){
		var btns = document.querySelectorAll('.card-btn');
		for (var i = 0; i < btns.length; i++) {
			if (btns[i].textContent.trim() === '点击预览附件简历') {
				btns[i].click();
				return JSON.stringify({success: true});
			}
		}
		return JSON.stringify({success: false, reason: 'no preview button found'});
	})()`

	raw, err = client.EvaluateValue(clickPreviewCode)
	if err != nil {
		return fmt.Errorf("click preview failed: %w", err)
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("preview failed: %s", result.Reason)
	}

	// Step 3: wait for iframe to load and extract PDF URL
	time.Sleep(2 * time.Second)

	extractCode := `(function(){
		var iframe = document.querySelector('.attachment-iframe');
		if (!iframe) iframe = document.querySelector('iframe[class*="attachment"]');
		if (!iframe) {
			var iframes = document.querySelectorAll('iframe');
			for (var i = 0; i < iframes.length; i++) {
				if (iframes[i].src.indexOf('pdf-viewer') > -1) {
					iframe = iframes[i];
					break;
				}
			}
		}
		if (!iframe) return JSON.stringify({found: false, reason: 'no attachment iframe found'});
		var src = iframe.src;
		var match = src.match(/url=([^&]+)/);
		var pdfUrl = match ? decodeURIComponent(match[1]) : '';
		return JSON.stringify({found: true, pdfUrl: pdfUrl, iframeSrc: src});
	})()`

	raw, err = client.EvaluateValue(extractCode)
	if err != nil {
		return fmt.Errorf("extract pdf url failed: %w", err)
	}
	var extractResult struct {
		Found     bool   `json:"found"`
		PDFUrl    string `json:"pdfUrl"`
		IFrameSrc string `json:"iframeSrc"`
		Reason    string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &extractResult); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}
	if !extractResult.Found {
		return fmt.Errorf("extract pdf url failed: %s", extractResult.Reason)
	}

	// Step 4: trigger browser download via anchor tag
	pdfURL := extractResult.PDFUrl
	if !strings.HasPrefix(pdfURL, "http") {
		pdfURL = "https://www.zhipin.com" + pdfURL
	}

	filename := sanitizeFilename(candidateName) + ".pdf"
	downloadCode := fmt.Sprintf(`(function(){
		var a = document.createElement('a');
		a.href = '%s';
		a.download = '%s';
		a.target = '_blank';
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		return JSON.stringify({triggered: true});
	})()`, strings.ReplaceAll(pdfURL, "'", "\\'"), strings.ReplaceAll(filename, "'", "\\'"))

	raw, err = client.EvaluateValue(downloadCode)
	if err != nil {
		return fmt.Errorf("trigger download failed: %w", err)
	}
	var dlResult struct {
		Triggered bool `json:"triggered"`
	}
	if err := json.Unmarshal(raw, &dlResult); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}
	if !dlResult.Triggered {
		return fmt.Errorf("download trigger failed")
	}

	// Step 5: wait for download and move to target dir
	time.Sleep(3 * time.Second)

	downloadedPath, err := findLatestDownload(filename)
	if err != nil {
		return fmt.Errorf("find downloaded file failed: %w", err)
	}

	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return err
	}
	outPath := filepath.Join(downloadDir, filename)
	if err := os.Rename(downloadedPath, outPath); err != nil {
		return fmt.Errorf("move file failed: %w", err)
	}

	return nil
}

func findLatestDownload(expectedName string) (string, error) {
	downloadDir := filepath.Join(os.Getenv("HOME"), "Downloads")
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return "", err
	}

	baseName := strings.TrimSuffix(expectedName, filepath.Ext(expectedName))
	var candidates []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, baseName) && strings.HasSuffix(name, ".pdf") {
			candidates = append(candidates, entry)
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no downloaded pdf found for %s", expectedName)
	}

	// Sort by modification time descending
	sort.Slice(candidates, func(i, j int) bool {
		infoI, _ := candidates[i].Info()
		infoJ, _ := candidates[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return filepath.Join(downloadDir, candidates[0].Name()), nil
}

func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|]`)
	return re.ReplaceAllString(name, "_")
}
