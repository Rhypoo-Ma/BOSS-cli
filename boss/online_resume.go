package boss

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Rhypoo-Ma/BOSS-cli/browser"
)

// ResumePreview contains the candidate info visible in the chat detail panel.
type ResumePreview struct {
	Name             string   `json:"name"`
	Age              string   `json:"age,omitempty"`
	WorkYears        string   `json:"work_years,omitempty"`
	EducationLevel   string   `json:"education_level,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	WorkExperience   []Work   `json:"work_experience,omitempty"`
	EducationHistory []Edu    `json:"education_history,omitempty"`
	Expectation      string   `json:"expectation,omitempty"`
	JobTitle         string   `json:"job_title,omitempty"`
	RawText          string   `json:"raw_text,omitempty"`
}

// FieldMatch describes a resume field that contains a keyword.
type FieldMatch struct {
	Field   string `json:"field"`
	Text    string `json:"text"`
	Keyword string `json:"keyword,omitempty"`
}

// ResumeSearchResult is the outcome of searching a resume preview for keywords.
type ResumeSearchResult struct {
	Keyword  string       `json:"keyword"`
	Keywords []string     `json:"keywords,omitempty"`
	Matched  bool         `json:"matched"`
	Count    int          `json:"count"`
	Matches  []FieldMatch `json:"matches,omitempty"`
}


type Work struct {
	Period      string `json:"period"`
	Company     string `json:"company"`
	Position    string `json:"position"`
	Description string `json:"description,omitempty"`
}

type Edu struct {
	Period   string `json:"period"`
	School   string `json:"school"`
	Major    string `json:"major,omitempty"`
	Degree   string `json:"degree,omitempty"`
}

// OpenOnlineResume clicks the candidate and opens the online resume dialog.
// It returns the job title shown in the chat detail panel before the dialog opens.
func OpenOnlineResume(client *browser.Client, name string) (string, error) {
	// Step 0: clean up any leftover resume dialogs from previous operations
	if err := cleanupResumeDialogs(client); err != nil {
		// Non-fatal; continue opening
	}

	// Step 1: click candidate in the list
	if err := clickCandidateByName(client, name); err != nil {
		return "", err
	}
	time.Sleep(800 * time.Millisecond)

	// Capture the job title from the detail panel while it is still visible.
	jobTitle := ""
	if preview, err := ExtractResumePreview(client); err == nil {
		jobTitle = strings.TrimSpace(preview.JobTitle)
	}

	// Step 2: click the online resume button
	code := `(function(){
		var btn = document.querySelector('.btn.resume-btn-online');
		if (!btn) return JSON.stringify({success: false, reason: 'online resume button not found'});
		btn.click();
		return JSON.stringify({success: true});
	})()`

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return jobTitle, fmt.Errorf("click online resume failed: %w", err)
	}
	var result struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return jobTitle, fmt.Errorf("parse failed: %w", err)
	}
	if !result.Success {
		return jobTitle, errors.New(result.Reason)
	}

	// Wait for the resume dialog to appear
	if err := client.WaitForSelector(".resume-container, .resume-common-dialog, .new-resume-online-main-ui", 5*time.Second); err != nil {
		return jobTitle, fmt.Errorf("resume dialog did not open: %w", err)
	}
	return jobTitle, nil
}

// CloseOnlineResume closes the currently active online resume dialog.
func CloseOnlineResume(client *browser.Client) error {
	return cleanupResumeDialogs(client)
}

// cleanupResumeDialogs hides all resume dialogs, including leftover ones.
func cleanupResumeDialogs(client *browser.Client) error {
	code := `(function(){
		return new Promise(function(resolve){
			var wraps = document.querySelectorAll('.dialog-wrap');
			for (var i = 0; i < wraps.length; i++) {
				wraps[i].classList.remove('active');
				wraps[i].classList.add('deactive');
				wraps[i].style.display = 'none';
				var d = wraps[i].querySelector('.resume-common-dialog, .resume-container, .boss-dialog__wrapper');
				if (d) {
					d.style.display = 'none';
					d.style.visibility = 'hidden';
					d.style.opacity = '0';
				}
			}
			setTimeout(function(){
				resolve(JSON.stringify({cleaned: wraps.length}));
			}, 100);
		});
	})()`

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return fmt.Errorf("close dialog failed: %w", err)
	}
	var result struct {
		Cleaned int    `json:"cleaned"`
		Reason  string `json:"reason,omitempty"`
	}
	json.Unmarshal(raw, &result)
	if result.Reason != "" {
		return errors.New(result.Reason)
	}
	return nil
}

// ScrollOnlineResume scrolls the online resume content down by the given pixels.
func ScrollOnlineResume(client *browser.Client, pixels int) error {
	code := fmt.Sprintf(`(function(){
		var container = document.querySelector('.resume-detail-wrap, .new-resume-online-main-ui, .resume-container, .boss-dialog__wrapper');
		if (!container) return JSON.stringify({error: 'resume container not found'});
		var scrollEl = container.querySelector('.resume-detail, .resume-content-wrap') || container;
		scrollEl.scrollTop += %d;
		return JSON.stringify({scrollTop: scrollEl.scrollTop, scrollHeight: scrollEl.scrollHeight, clientHeight: scrollEl.clientHeight});
	})()`, pixels)

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return fmt.Errorf("scroll failed: %w", err)
	}
	var r struct {
		Error string `json:"error,omitempty"`
	}
	json.Unmarshal(raw, &r)
	if r.Error != "" {
		return errors.New(r.Error)
	}
	return nil
}

// ExtractResumePreview extracts structured resume info from the chat detail panel.
func ExtractResumePreview(client *browser.Client) (*ResumePreview, error) {
	code := `(function(){
		var detail = document.querySelector('.chat-conversation') || document.querySelector('.conversation-box') || document.querySelector('.chat-conversation-detail');
		if (!detail) return JSON.stringify({error: 'detail panel not found'});
		var top = detail.querySelector('.base-info-single-top-detail, .base-info');
		var basic = top ? top.innerText.split('\n').map(function(s){ return s.trim(); }).filter(function(s){ return s.length > 0; }) : [];
		var timeList = detail.querySelector('.experience-content.time-list');
		var detailList = detail.querySelector('.experience-content.detail-list');
		var times = [];
		var details = [];
		if (timeList) {
			var tItems = timeList.querySelectorAll(':scope > ul > li, :scope > li');
			for (var i=0;i<tItems.length;i++){ var t=tItems[i].textContent.trim(); if (t) times.push(t); }
		}
		if (detailList) {
			var dItems = detailList.querySelectorAll(':scope > ul > li, :scope > li');
			for (var i=0;i<dItems.length;i++){ var t=dItems[i].textContent.trim(); if (t) details.push(t); }
		}
		var expectation = '';
		var jobTitle = '';
		var all = detail.querySelectorAll('*');
		for (var i=0;i<all.length;i++){
			var t = all[i].textContent.trim();
			if (t.indexOf('期望：') === 0) expectation = t;
			if (t.indexOf('沟通职位：') === 0) jobTitle = t;
		}
		return JSON.stringify({basic: basic, times: times, details: details, expectation: expectation, jobTitle: jobTitle, rawText: detail.innerText});
	})()`

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return nil, fmt.Errorf("extract failed: %w", err)
	}
	var data struct {
		Basic       []string `json:"basic"`
		Times       []string `json:"times"`
		Details     []string `json:"details"`
		Expectation string   `json:"expectation"`
		JobTitle    string   `json:"jobTitle"`
		RawText     string   `json:"rawText"`
		Error       string   `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}
	if data.Error != "" {
		return nil, errors.New(data.Error)
	}
	return buildResumePreview(data.Basic, data.Times, data.Details, data.Expectation, data.JobTitle, data.RawText), nil
}

func clickCandidateByName(client *browser.Client, name string) error {
	escaped := strings.ReplaceAll(name, "'", "\\'")
	code := fmt.Sprintf(`(function(){
		var items = document.querySelectorAll('.geek-item-wrap');
		for (var i = 0; i < items.length; i++) {
			if (items[i].textContent.indexOf('%s') > -1) {
				items[i].click();
				return JSON.stringify({success: true, index: i});
			}
		}
		return JSON.stringify({success: false, reason: 'candidate not found in visible list'});
	})()`, escaped)

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return fmt.Errorf("click candidate failed: %w", err)
	}
	var result struct {
		Success bool   `json:"success"`
		Index   int    `json:"index,omitempty"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}
	if !result.Success {
		return errors.New(result.Reason)
	}
	return nil
}

func buildResumePreview(basic, times, details []string, expectation, jobTitle, rawText string) *ResumePreview {
	rp := &ResumePreview{
		RawText: rawText,
	}

	if len(basic) > 0 {
		rp.Name = basic[0]
	}
	if len(basic) > 1 {
		rp.Age = basic[1]
	}
	if len(basic) > 2 {
		rp.WorkYears = basic[2]
	}
	if len(basic) > 3 {
		rp.EducationLevel = basic[3]
	}
	if len(basic) > 4 {
		for _, tag := range basic[4:] {
			if tag != "" && !strings.Contains(tag, "简历") && !strings.Contains(tag, "职位") {
				rp.Tags = append(rp.Tags, tag)
			}
		}
	}

	// Pair times with details by index
	for i := 0; i < len(times) && i < len(details); i++ {
		detail := details[i]
		parts := strings.Split(detail, "·")
		if len(parts) < 2 {
			continue
		}
		// Determine if it's education by looking for school/degree keywords
		if isEducationDetail(detail) {
			edu := Edu{Period: times[i]}
			edu.School = strings.TrimSpace(parts[0])
			if len(parts) >= 2 {
				edu.Major = strings.TrimSpace(parts[1])
			}
			if len(parts) >= 3 {
				edu.Degree = strings.TrimSpace(parts[2])
			}
			rp.EducationHistory = append(rp.EducationHistory, edu)
		} else {
			work := Work{
				Period:   times[i],
				Company:  strings.TrimSpace(parts[0]),
				Position: strings.TrimSpace(parts[1]),
			}
			rp.WorkExperience = append(rp.WorkExperience, work)
		}
	}

	if strings.HasPrefix(expectation, "期望：") {
		rp.Expectation = strings.TrimPrefix(expectation, "期望：")
	} else {
		rp.Expectation = expectation
	}
	if strings.HasPrefix(jobTitle, "沟通职位：") {
		rp.JobTitle = strings.TrimPrefix(jobTitle, "沟通职位：")
	} else {
		rp.JobTitle = jobTitle
	}

	return rp
}

func isEducationDetail(s string) bool {
	keywords := []string{"大学", "学院", "本科", "硕士", "博士", "大专", "研究生", "高中", "中专"}
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// SearchResume searches a resume preview for the given keywords (case-insensitive).
// It checks all structured fields and the raw text snapshot.
// When excludeJobTitle is true, matches that only come from the job title are ignored.
func SearchResume(preview *ResumePreview, keywords []string, excludeJobTitle bool) *ResumeSearchResult {
	result := &ResumeSearchResult{}
	hasKeyword := false
	for _, k := range keywords {
		if strings.TrimSpace(k) != "" {
			hasKeyword = true
			break
		}
	}
	if !hasKeyword || preview == nil {
		return result
	}
	result.Keyword = keywords[0]
	result.Keywords = keywords

	jobTitle := strings.TrimSpace(preview.JobTitle)
	isJobTitleOnly := func(field, text string) bool {
		if !excludeJobTitle {
			return false
		}
		if field == "job_title" {
			return true
		}
		if field == "raw_text" && jobTitle != "" && strings.TrimSpace(text) == jobTitle {
			return true
		}
		return false
	}

	add := func(field, text string) {
		if text == "" {
			return
		}
		if isJobTitleOnly(field, text) {
			return
		}
		lower := strings.ToLower(text)
		for _, kw := range keywords {
			if keywordMatches(lower, kw) {
				result.Matches = append(result.Matches, FieldMatch{Field: field, Text: text, Keyword: kw})
				return
			}
		}
	}

	add("name", preview.Name)
	add("age", preview.Age)
	add("work_years", preview.WorkYears)
	add("education_level", preview.EducationLevel)
	add("expectation", preview.Expectation)
	add("job_title", preview.JobTitle)
	for _, tag := range preview.Tags {
		add("tag", tag)
	}
	for i, w := range preview.WorkExperience {
		add(fmt.Sprintf("work_experience.%d.period", i), w.Period)
		add(fmt.Sprintf("work_experience.%d.company", i), w.Company)
		add(fmt.Sprintf("work_experience.%d.position", i), w.Position)
		add(fmt.Sprintf("work_experience.%d.description", i), w.Description)
	}
	for i, e := range preview.EducationHistory {
		add(fmt.Sprintf("education_history.%d.period", i), e.Period)
		add(fmt.Sprintf("education_history.%d.school", i), e.School)
		add(fmt.Sprintf("education_history.%d.major", i), e.Major)
		add(fmt.Sprintf("education_history.%d.degree", i), e.Degree)
	}
	// Raw text is checked last so it can surface fields not parsed above.
	add("raw_text", preview.RawText)

	result.Count = len(result.Matches)
	result.Matched = result.Count > 0
	return result
}

// keywordMatches checks whether text contains the keyword, including OCR-tolerant variants.
func keywordMatches(lowerText, keyword string) bool {
	kw := strings.ToLower(keyword)
	if strings.Contains(lowerText, kw) {
		return true
	}
	// Vision OCR often reads uppercase "AI" as "Al".
	if kw == "ai" && strings.Contains(lowerText, "al") {
		return true
	}
	return false
}
