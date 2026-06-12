package boss

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Rhypoo-Ma/BOSS-cli/browser"
)

type Job struct {
	Name     string `json:"name"`
	Location string `json:"location,omitempty"`
	Salary   string `json:"salary,omitempty"`
	Closed   bool   `json:"closed,omitempty"`
}

// jobDropdownSelector returns a JavaScript expression that tries multiple selectors.
func jobDropdownSelector() string {
	return `document.querySelector('.chat-select-job, .ui-dropmenu-label, [class*="job-select"], [class*="select-job"], .job-dropdown')`
}

// jobItemSelector returns a JavaScript expression that matches job items in the dropdown.
func jobItemSelector() string {
	return `document.querySelectorAll('.ui-dropmenu-item, .dropdown-item, [class*="dropmenu"] li, [class*="dropdown"] li, .select-job-list li, [class*="job-select"] li, [class*="job-dropdown"] li')`
}

func ListJobs(client *browser.Client) ([]Job, error) {
	// Click the job dropdown to expand it
	clickCode := fmt.Sprintf(`(function(){
		var el = %s;
		if (el) { el.click(); return JSON.stringify({clicked:true}); }
		return JSON.stringify({clicked:false});
	})()`, jobDropdownSelector())

	raw, err := client.EvaluateValue(clickCode)
	if err != nil {
		return nil, fmt.Errorf("click dropdown failed: %w", err)
	}
	var clickResult struct {
		Clicked bool `json:"clicked"`
	}
	json.Unmarshal(raw, &clickResult)
	if clickResult.Clicked {
		// Wait for dropdown items to render
		if err := client.WaitFor(fmt.Sprintf(`%s.length > 0`, jobItemSelector()), 5*time.Second, 200*time.Millisecond); err != nil {
			return nil, fmt.Errorf("dropdown did not open: %w", err)
		}
	}

	// Extract job list items
	listCode := fmt.Sprintf(`(function(){
		var items = %s;
		var jobs = [];
		for (var i = 0; i < items.length; i++) {
			var t = items[i].textContent.trim();
			if (t && t.length > 0 && t !== '全部职位') {
				jobs.push(t);
			}
		}
		return JSON.stringify(jobs);
	})()`, jobItemSelector())

	raw, err = client.EvaluateValue(listCode)
	if err != nil {
		return nil, fmt.Errorf("evaluate failed: %w", err)
	}

	var texts []string
	if err := json.Unmarshal(raw, &texts); err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	var jobs []Job
	for _, text := range texts {
		if isUserMenuItem(text) {
			continue
		}
		job := parseJobText(text)
		if job.Name != "" {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func SwitchJob(client *browser.Client, jobName string) error {
	return browser.Retry(func() error {
		return switchJobOnce(client, jobName)
	}, 3, 1*time.Second)
}

func switchJobOnce(client *browser.Client, jobName string) error {
	// Ensure dropdown is open
	clickCode := fmt.Sprintf(`(function(){
		var el = %s;
		if (el) { el.click(); return JSON.stringify({clicked:true}); }
		return JSON.stringify({clicked:false});
	})()`, jobDropdownSelector())

	raw, err := client.EvaluateValue(clickCode)
	if err != nil {
		return fmt.Errorf("click dropdown failed: %w", err)
	}
	var clickResult struct {
		Clicked bool `json:"clicked"`
	}
	json.Unmarshal(raw, &clickResult)
	if clickResult.Clicked {
		if err := client.WaitFor(fmt.Sprintf(`%s.length > 0`, jobItemSelector()), 3*time.Second, 200*time.Millisecond); err != nil {
			return fmt.Errorf("dropdown did not open: %w", err)
		}
	}

	// Find and click the job item
	code := fmt.Sprintf(`(function(){
		var items = %s;
		for (var i = 0; i < items.length; i++) {
			var t = items[i].textContent.trim();
			if (t.indexOf('%s') > -1) {
				items[i].click();
				return JSON.stringify({success: true, matched: t});
			}
		}
		return JSON.stringify({success: false, reason: 'job not found'});
	})()`, jobItemSelector(), strings.ReplaceAll(jobName, "'", "\\'"))

	raw, err = client.EvaluateValue(code)
	if err != nil {
		return fmt.Errorf("evaluate failed: %w", err)
	}
	var result struct {
		Success bool   `json:"success"`
		Matched string `json:"matched,omitempty"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("switch job failed: %s", result.Reason)
	}

	// Wait for the dropdown to close and the selected job to appear in the UI
	if err := client.WaitForText(jobDropdownSelector(), jobName, 5*time.Second); err != nil {
		return fmt.Errorf("job label did not update to %q: %w", jobName, err)
	}

	// Wait for candidate list to load for the new job
	if err := client.WaitForSelector(".geek-item-wrap", 5*time.Second); err != nil {
		return fmt.Errorf("candidate list did not load after switching job: %w", err)
	}
	return nil
}

func isUserMenuItem(text string) bool {
	noise := []string{
		"个人中心", "账号设置", "钱包/发票", "首充优惠", "直豆",
		"桌面客户端", "最佳招聘官", "退出登录", "不合适", "牛人发起",
		"我发起", "道具来源", "群聊", "管理分组", "新建分组",
		"这里是根据你的消息过滤设置过滤掉的牛人消息",
	}
	for _, n := range noise {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

func parseJobText(text string) Job {
	closed := strings.Contains(text, "（关闭）") || strings.Contains(text, "(关闭)")
	name := text
	var location, salary string

	parts := strings.Split(text, " _ ")
	if len(parts) >= 2 {
		name = strings.TrimSpace(parts[0])
		rest := strings.TrimSpace(parts[1])
		spaceIdx := strings.LastIndex(rest, " ")
		if spaceIdx > 0 {
			location = strings.TrimSpace(rest[:spaceIdx])
			salary = strings.TrimSpace(rest[spaceIdx:])
		} else {
			location = rest
		}
	}

	return Job{
		Name:     name,
		Location: location,
		Salary:   salary,
		Closed:   closed,
	}
}
