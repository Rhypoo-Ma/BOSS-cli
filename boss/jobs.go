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

// jobDropdownTrigger returns the CSS selector for the job dropdown trigger.
func jobDropdownTrigger() string {
	return ".chat-top-job, .chat-select-job"
}

// jobDropdownLabel returns the CSS selector for the current job label text.
// Scoped inside .chat-top-job to avoid matching the user profile menu.
func jobDropdownLabel() string {
	return ".chat-top-job .chat-select-job"
}

// jobItemSelector returns the CSS selector for job items inside the dropdown.
func jobItemSelector() string {
	return ".ui-dropmenu-item, .dropdown-item, [class*=\"dropmenu\"] li, [class*=\"dropdown\"] li, .select-job-list li, [class*=\"job-select\"] li, [class*=\"job-dropdown\"] li"
}

func ListJobs(client *browser.Client) ([]Job, error) {
	if err := ensureDropdownOpen(client); err != nil {
		return nil, err
	}

	// Wait for dropdown items to render
	if err := client.WaitFor(fmt.Sprintf(`document.querySelectorAll('%s').length > 0`, jobItemSelector()), 5*time.Second, 200*time.Millisecond); err != nil {
		return nil, fmt.Errorf("dropdown did not open: %w", err)
	}

	// Extract job list items
	listCode := fmt.Sprintf(`(function(){
		var items = document.querySelectorAll('%s');
		var jobs = [];
		for (var i = 0; i < items.length; i++) {
			var t = items[i].textContent.trim();
			if (t && t.length > 0 && t !== '全部职位') {
				jobs.push(t);
			}
		}
		return JSON.stringify(jobs);
	})()`, jobItemSelector())

	raw, err := client.EvaluateValue(listCode)
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
	// Fast path: already on target job
	if isCurrentJob(client, jobName) {
		return nil
	}

	// Make sure dropdown is open; close and reopen if needed
	if err := ensureDropdownOpen(client); err != nil {
		return fmt.Errorf("open dropdown failed: %w", err)
	}

	// Find and click the target job item
	code := fmt.Sprintf(`(function(){
		var items = document.querySelectorAll('%s');
		for (var i = 0; i < items.length; i++) {
			var t = items[i].textContent.trim();
			if (t.indexOf('%s') > -1) {
				items[i].click();
				return JSON.stringify({success: true, matched: t});
			}
		}
		return JSON.stringify({success: false, reason: 'job not found'});
	})()`, jobItemSelector(), strings.ReplaceAll(jobName, "'", "\\'"))

	raw, err := client.EvaluateValue(code)
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

	// Wait for the dropdown to close and the selected job label to update
	if err := client.WaitForText(jobDropdownLabel(), jobName, 5*time.Second); err != nil {
		return fmt.Errorf("job label did not update to %q: %w", jobName, err)
	}

	// Wait for candidate list to load for the new job
	if err := client.WaitForSelector(".geek-item-wrap", 5*time.Second); err != nil {
		return fmt.Errorf("candidate list did not load after switching job: %w", err)
	}
	return nil
}

func isCurrentJob(client *browser.Client, jobName string) bool {
	code := fmt.Sprintf(`(function(){
		var el = document.querySelector('%s');
		return JSON.stringify({current: el ? el.textContent.trim() : ''});
	})()`, jobDropdownLabel())
	raw, err := client.EvaluateValue(code)
	if err != nil {
		return false
	}
	var r struct {
		Current string `json:"current"`
	}
	json.Unmarshal(raw, &r)
	return strings.Contains(r.Current, jobName)
}

func ensureDropdownOpen(client *browser.Client) error {
	// Check if any job dropdown is already open
	checkCode := `(function(){
		var open = document.querySelector('.chat-top-job .ui-dropmenu-list, .chat-top-job [class*="dropmenu-list"], .chat-top-job.active');
		return JSON.stringify({open: !!open});
	})()`
	raw, err := client.EvaluateValue(checkCode)
	if err != nil {
		return fmt.Errorf("check dropdown failed: %w", err)
	}
	var check struct {
		Open bool `json:"open"`
	}
	json.Unmarshal(raw, &check)

	// If already open, click trigger again to close it, then reopen
	if check.Open {
		closeCode := fmt.Sprintf(`(function(){
			var el = document.querySelector('%s');
			if (el) { el.click(); }
			return JSON.stringify({clicked: true});
		})()`, jobDropdownTrigger())
		client.EvaluateValue(closeCode)
		time.Sleep(300 * time.Millisecond)
	}

	// Click to open
	openCode := fmt.Sprintf(`(function(){
		var el = document.querySelector('%s');
		if (!el) return JSON.stringify({clicked: false});
		el.click();
		return JSON.stringify({clicked: true});
	})()`, jobDropdownTrigger())

	raw, err = client.EvaluateValue(openCode)
	if err != nil {
		return fmt.Errorf("click dropdown failed: %w", err)
	}
	var result struct {
		Clicked bool `json:"clicked"`
	}
	json.Unmarshal(raw, &result)
	if !result.Clicked {
		return fmt.Errorf("job dropdown trigger not found")
	}

	// Wait for dropdown items to render
	return client.WaitFor(fmt.Sprintf(`document.querySelectorAll('%s').length > 0`, jobItemSelector()), 3*time.Second, 200*time.Millisecond)
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
