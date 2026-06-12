package boss

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Rhypoo-Ma/BOSS-cli/browser"
)

// OCRFieldMatch describes a text line that matched a keyword.
type OCRFieldMatch struct {
	Text    string `json:"text"`
	Keyword string `json:"keyword,omitempty"`
}

// OCRSearchResult holds keyword search results over OCR-extracted resume text.
type OCRSearchResult struct {
	Keyword  string          `json:"keyword"`
	Keywords []string        `json:"keywords,omitempty"`
	Matched  bool            `json:"matched"`
	Count    int             `json:"count"`
	Text     string          `json:"text,omitempty"`
	Matches  []OCRFieldMatch `json:"matches,omitempty"`
}

// SearchResumeWithOCR opens the candidate's online resume, screenshots the page,
// extracts visible text with OCR, and searches for the keywords.
// When excludeJobTitle is true, matches that only reflect the job title are ignored.
func SearchResumeWithOCR(client *browser.Client, name string, keywords []string, excludeJobTitle bool) (*OCRSearchResult, error) {
	result := &OCRSearchResult{}
	hasKeyword := false
	for _, k := range keywords {
		if strings.TrimSpace(k) != "" {
			hasKeyword = true
			break
		}
	}
	if !hasKeyword {
		return result, nil
	}
	result.Keyword = keywords[0]
	result.Keywords = keywords

	jobTitle, err := OpenOnlineResume(client, name)
	if err != nil {
		return nil, fmt.Errorf("open resume failed: %w", err)
	}
	// Wait for the canvas/dialog to settle.
	time.Sleep(1200 * time.Millisecond)

	screenshotPath, err := client.Screenshot("png")
	if err != nil {
		_ = CloseOnlineResume(client)
		return nil, fmt.Errorf("screenshot failed: %w", err)
	}

	// Prefer the chat detail panel which currently renders the resume summary.
	rect, err := client.GetBoundingRect(".chat-conversation, .conversation-box, .chat-conversation-detail")
	if err != nil || rect.Empty() {
		// Fall back to the whole screenshot if the panel rect is unavailable.
		rect = image.Rect(0, 0, 0, 0)
	}

	// Screenshot is captured at device pixels; CSS rects need scaling by DPR.
	dpr, _ := getDevicePixelRatio(client)
	if dpr > 1 && !rect.Empty() {
		rect = scaleRect(rect, dpr)
	}

	ocrPath := screenshotPath
	if !rect.Empty() {
		croppedPath := screenshotPath + ".crop.png"
		if err := cropPNG(screenshotPath, croppedPath, rect); err == nil {
			ocrPath = croppedPath
		}
	}

	text, err := runOCR(ocrPath)
	if err != nil {
		_ = CloseOnlineResume(client)
		return nil, fmt.Errorf("ocr failed: %w", err)
	}
	text = cleanOCRText(text)

	// Supplement with the structured panel text in case OCR missed key fields.
	if preview, err := ExtractResumePreview(client); err == nil && preview.RawText != "" {
		text = text + "\n" + preview.RawText
	}

	_ = CloseOnlineResume(client)

	if jobTitle == "" {
		jobTitle = extractJobTitleFromText(text)
	}
	result.Text = truncate(text, 2000)
	result.Matches = findKeywordLines(text, keywords, excludeJobTitle, jobTitle)
	result.Count = len(result.Matches)
	result.Matched = result.Count > 0
	for i := range result.Matches {
		result.Matches[i].Text = truncate(result.Matches[i].Text, 200)
	}
	if len(result.Matches) > 20 {
		result.Matches = result.Matches[:20]
	}
	return result, nil
}

func findKeywordLines(text string, keywords []string, excludeJobTitle bool, jobTitle string) []OCRFieldMatch {
	if len(keywords) == 0 || text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	seen := map[string]bool{}
	var matches []OCRFieldMatch
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if excludeJobTitle && isJobTitleLine(line, jobTitle) {
			continue
		}
		lower := strings.ToLower(line)
		for _, kw := range keywords {
			if keywordMatches(lower, kw) {
				if !seen[line] {
					seen[line] = true
					matches = append(matches, OCRFieldMatch{Text: line, Keyword: kw})
				}
				break
			}
		}
	}
	return matches
}

// extractJobTitleFromText parses the job title from common BOSS UI labels in OCR text.
func extractJobTitleFromText(text string) string {
	markers := []string{"沟通职位：", "沟通的职位", "沟通职位"}
	for _, marker := range markers {
		idx := strings.Index(text, marker)
		if idx < 0 {
			continue
		}
		start := idx + len(marker)
		// Skip separators commonly introduced by OCR/UI.
		for start < len(text) {
			r, size := utf8.DecodeRuneInString(text[start:])
			if r == ' ' || r == '\t' || r == '-' || r == '：' || r == ':' {
				start += size
			} else {
				break
			}
		}
		end := start
		for end < len(text) && text[end] != '\n' && text[end] != '\r' {
			end++
		}
		title := strings.TrimSpace(text[start:end])
		// Stop at obvious trailing punctuation/labels.
		if cut := strings.IndexAny(title, "（()【[｜|"); cut > 0 {
			title = strings.TrimSpace(title[:cut])
		}
		if title != "" {
			return title
		}
	}
	return ""
}

// isJobTitleLine detects lines that mostly consist of the job title metadata.
func isJobTitleLine(line, jobTitle string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	// BOSS UI labels that introduce the job title.
	if strings.Contains(lower, "沟通职位") || strings.Contains(lower, "沟通的职位") {
		return true
	}
	if jobTitle == "" {
		return false
	}
	// A line that is exactly the job title.
	if trimmed == jobTitle {
		return true
	}
	// Any line that contains the full job title is treated as job-title noise.
	if jobTitle != "" && strings.Contains(lower, strings.ToLower(jobTitle)) {
		return true
	}
	return false
}

func getDevicePixelRatio(client *browser.Client) (int, error) {
	raw, err := client.EvaluateValue(`JSON.stringify({dpr: window.devicePixelRatio || 1})`)
	if err != nil {
		return 1, err
	}
	var r struct {
		DPR float64 `json:"dpr"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return 1, err
	}
	if r.DPR < 1 {
		return 1, nil
	}
	return int(r.DPR + 0.5), nil
}

func scaleRect(r image.Rectangle, factor int) image.Rectangle {
	return image.Rect(r.Min.X*factor, r.Min.Y*factor, r.Max.X*factor, r.Max.Y*factor)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func runOCR(imagePath string) (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine script path")
	}
	script := filepath.Join(filepath.Dir(file), "..", "scripts", "ocr.swift")
	sdk := "/Library/Developer/CommandLineTools/SDKs/MacOSX15.5.sdk"
	out, err := exec.Command("swift", "-sdk", sdk, script, imagePath).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("ocr script failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("ocr script failed: %w", err)
	}
	return string(out), nil
}

func cleanOCRText(text string) string {
	// Vision OCR sometimes drops the leading "A" from "Agent".
	text = strings.ReplaceAll(text, "gent/", "agent/")
	text = strings.ReplaceAll(text, "Gent/", "Agent/")
	text = strings.ReplaceAll(text, "gent ", "agent ")
	text = strings.ReplaceAll(text, "Gent ", "Agent ")
	text = strings.ReplaceAll(text, "gent\n", "agent\n")
	text = strings.ReplaceAll(text, "Gent\n", "Agent\n")
	return text
}

func cropPNG(inputPath, outputPath string, rect image.Rectangle) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	sub, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	if !ok {
		return fmt.Errorf("image type does not support sub-image cropping")
	}
	cropped := sub.SubImage(rect)
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, cropped)
}
