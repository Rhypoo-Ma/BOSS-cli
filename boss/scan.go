package boss

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rhypoo-Ma/BOSS-cli/browser"
)

// ScanResult records the outcome of scanning one candidate.
type ScanResult struct {
	Candidate Candidate `json:"candidate"`
	Matched   bool      `json:"matched"`
	Keyword   string    `json:"keyword,omitempty"`
	Matches   int       `json:"matches"`
	GradYear  int       `json:"grad_year,omitempty"`
	School    string    `json:"school,omitempty"`
	Sent      bool      `json:"sent"`
	Error     string    `json:"error,omitempty"`
}

// ScanResumes switches to the given job/filter, collects candidates, and for each one
// searches the online resume for keywords.
//   - If minGrade > 0, the graduation year must be >= minGrade.
//   - If schoolTier is set (c9/985/overseas) or schools is non-empty, the resume text
//     must contain one of the corresponding school keywords.
//   - If names is non-empty, only candidates in that set are scanned.
// When all enabled conditions pass and message is non-empty, it sends the message.
func ScanResumes(
	client *browser.Client,
	jobName, filterStatus string,
	unreadOnly bool,
	keywords []string,
	message string,
	useOCR, excludeJobTitle bool,
	max int,
	minGrade int,
	names []string,
	schoolTier string,
	schools []string,
) ([]ScanResult, error) {
	if _, err := SwitchJobWithFilters(client, jobName, filterStatus, unreadOnly); err != nil {
		return nil, fmt.Errorf("switch job failed: %w", err)
	}

	candidates, err := ListCandidates(client, filterStatus, true, max)
	if err != nil {
		return nil, fmt.Errorf("list candidates failed: %w", err)
	}

	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[strings.TrimSpace(n)] = true
	}

	// Build the effective school keyword list from tier + custom schools.
	 effectiveSchools := append([]string{}, schools...)
	if tierKeywords := SchoolKeywords(schoolTier); tierKeywords != nil {
		effectiveSchools = append(effectiveSchools, tierKeywords...)
	}

	var results []ScanResult
	for _, cand := range candidates {
		if len(nameSet) > 0 && !nameSet[cand.Name] {
			continue
		}
		res := ScanResult{Candidate: cand}
		matched, keyword, matchCount, text, scanErr := scanOneWithText(client, cand.Name, keywords, useOCR, excludeJobTitle)
		if scanErr != nil {
			res.Error = scanErr.Error()
			results = append(results, res)
			continue
		}
		res.Matched = matched
		res.Keyword = keyword
		res.Matches = matchCount

		if text != "" {
			if minGrade > 0 {
				res.GradYear = extractGradYear(text)
			}
			if len(effectiveSchools) > 0 {
				res.School = extractSchool(text, effectiveSchools)
			}
		}

		eligible := matched
		if minGrade > 0 {
			eligible = eligible && res.GradYear > 0 && res.GradYear >= minGrade
		}
		if len(effectiveSchools) > 0 {
			eligible = eligible && res.School != ""
		}

		if eligible && strings.TrimSpace(message) != "" {
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

func scanOneWithText(client *browser.Client, name string, keywords []string, useOCR, excludeJobTitle bool) (bool, string, int, string, error) {
	if useOCR {
		result, err := SearchResumeWithOCR(client, name, keywords, excludeJobTitle)
		if err != nil {
			return false, "", 0, "", err
		}
		return result.Matched, result.Keyword, result.Count, result.Text, nil
	}

	if _, err := OpenOnlineResume(client, name); err != nil {
		return false, "", 0, "", err
	}
	preview, err := ExtractResumePreview(client)
	_ = CloseOnlineResume(client)
	if err != nil {
		return false, "", 0, "", err
	}
	result := SearchResume(preview, keywords, excludeJobTitle)
	return result.Matched, result.Keyword, result.Count, preview.RawText, nil
}

var gradYearPatterns = []*regexp.Regexp{
	regexp.MustCompile(`20(\d{2})届`),
	regexp.MustCompile(`(\d{2})届`),
	regexp.MustCompile(`20(\d{2})年毕业`),
	regexp.MustCompile(`(\d{2})年毕业`),
	regexp.MustCompile(`20(\d{2})年应届`),
	regexp.MustCompile(`(\d{2})年应届`),
	regexp.MustCompile(`20(\d{2})届应届`),
	regexp.MustCompile(`(\d{2})届应届`),
	regexp.MustCompile(`预计20(\d{2})`),
	regexp.MustCompile(`(\d{4})年毕业`),
}

// extractGradYear parses common graduation year expressions from resume text.
// It returns 0 if no year is found.
func extractGradYear(text string) int {
	lower := strings.ToLower(text)
	for _, re := range gradYearPatterns {
		matches := re.FindStringSubmatch(lower)
		if len(matches) < 2 {
			continue
		}
		year, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}
		if year < 100 {
			year += 2000
		}
		if year >= 2020 && year <= 2035 {
			return year
		}
	}
	return 0
}

// extractSchool returns the first school keyword found in the resume text.
func extractSchool(text string, schools []string) string {
	lower := strings.ToLower(text)
	for _, s := range schools {
		if strings.Contains(lower, strings.ToLower(s)) {
			return s
		}
	}
	return ""
}

// extractSchoolTier returns the tier label (c9/985/overseas) matched in the text, if any.
func extractSchoolTier(text string) string {
	if school := extractSchool(text, c9Schools); school != "" {
		return "c9"
	}
	if school := extractSchool(text, project985Schools); school != "" {
		return "985"
	}
	if IsOverseasSchool(text) {
		return "overseas"
	}
	return ""
}
