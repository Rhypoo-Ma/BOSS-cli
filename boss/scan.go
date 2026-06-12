package boss

import (
	"fmt"
	"strings"

	"github.com/Rhypoo-Ma/BOSS-cli/browser"
)

// ScanResult records the outcome of scanning one candidate.
type ScanResult struct {
	Candidate Candidate `json:"candidate"`
	Matched   bool      `json:"matched"`
	Keyword   string    `json:"keyword,omitempty"`
	Matches   int       `json:"matches"`
	Sent      bool      `json:"sent"`
	Error     string    `json:"error,omitempty"`
}

// ScanResumes switches to the given job/filter, collects candidates, and for each one
// searches the online resume for keywords. If a keyword matches and message is non-empty,
// it sends the message to the candidate.
func ScanResumes(
	client *browser.Client,
	jobName, filterStatus string,
	unreadOnly bool,
	keywords []string,
	message string,
	useOCR, excludeJobTitle bool,
	max int,
) ([]ScanResult, error) {
	if _, err := SwitchJobWithFilters(client, jobName, filterStatus, unreadOnly); err != nil {
		return nil, fmt.Errorf("switch job failed: %w", err)
	}

	candidates, err := ListCandidates(client, filterStatus, true, max)
	if err != nil {
		return nil, fmt.Errorf("list candidates failed: %w", err)
	}

	var results []ScanResult
	for _, cand := range candidates {
		res := ScanResult{Candidate: cand}
		matched, keyword, matchCount, scanErr := scanOne(client, cand.Name, keywords, useOCR, excludeJobTitle)
		if scanErr != nil {
			res.Error = scanErr.Error()
			results = append(results, res)
			continue
		}
		res.Matched = matched
		res.Keyword = keyword
		res.Matches = matchCount

		if matched && strings.TrimSpace(message) != "" {
			if err := SendMessage(client, cand.Name, message); err != nil {
				res.Error = fmt.Sprintf("send message failed: %v", err)
			} else {
				res.Sent = true
			}
		}
		results = append(results, res)
	}
	return results, nil
}

func scanOne(client *browser.Client, name string, keywords []string, useOCR, excludeJobTitle bool) (bool, string, int, error) {
	if useOCR {
		result, err := SearchResumeWithOCR(client, name, keywords, excludeJobTitle)
		if err != nil {
			return false, "", 0, err
		}
		return result.Matched, result.Keyword, result.Count, nil
	}

	if _, err := OpenOnlineResume(client, name); err != nil {
		return false, "", 0, err
	}
	preview, err := ExtractResumePreview(client)
	_ = CloseOnlineResume(client)
	if err != nil {
		return false, "", 0, err
	}
	result := SearchResume(preview, keywords, excludeJobTitle)
	return result.Matched, result.Keyword, result.Count, nil
}
