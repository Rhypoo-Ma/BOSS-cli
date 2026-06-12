package boss

import (
	"github.com/Rhypoo-Ma/BOSS-cli/browser"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Job struct {
	Name     string `json:"name"`
	Location string `json:"location,omitempty"`
	Salary   string `json:"salary,omitempty"`
	Closed   bool   `json:"closed,omitempty"`
}

func ListJobs(client *browser.Client) ([]Job, error) {
	// Click the job dropdown to expand it
	clickCode := `(function(){
		var el = document.querySelector('.chat-select-job, .ui-dropmenu-label, [class*="job-select"]');
		if (el) { el.click(); return JSON.stringify({clicked:true}); }
		return JSON.stringify({clicked:false});
	})()`

	raw, err := client.EvaluateValue(clickCode)
	if err != nil {
		return nil, fmt.Errorf("click dropdown failed: %w", err)
	}
	var clickResult struct {
		Clicked bool `json:"clicked"`
	}
	json.Unmarshal(raw, &clickResult)
	if clickResult.Clicked {
		time.Sleep(3 * time.Second)
	}

	// Extract job list items
	listCode := `(function(){
		var items = document.querySelectorAll('.ui-dropmenu-item, .dropdown-item, [class*="dropmenu"] li, [class*="dropdown"] li, .select-job-list li');
		var jobs = [];
		for (var i = 0; i < items.length; i++) {
			var t = items[i].textContent.trim();
			if (t && t.length > 0 && t !== '全部职位') {
				jobs.push(t);
			}
		}
		return JSON.stringify(jobs);
	})()`

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
		// Skip user menu items
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
	// First expand the dropdown
	clickCode := `(function(){
		var el = document.querySelector('.chat-select-job, .ui-dropmenu-label, [class*="job-select"]');
		if (el) { el.click(); return JSON.stringify({clicked:true}); }
		return JSON.stringify({clicked:false});
	})()`

	raw, err := client.EvaluateValue(clickCode)
	if err != nil {
		return fmt.Errorf("click dropdown failed: %w", err)
	}
	var clickResult struct {
		Clicked bool `json:"clicked"`
	}
	json.Unmarshal(raw, &clickResult)
	if clickResult.Clicked {
		time.Sleep(800 * time.Millisecond)
	}

	// Find and click the job item
	code := fmt.Sprintf(`(function(){
		var items = document.querySelectorAll('.ui-dropmenu-item, .dropdown-item, [class*="dropmenu"] li, [class*="dropdown"] li, .select-job-list li');
		for (var i = 0; i < items.length; i++) {
			var t = items[i].textContent.trim();
			if (t.indexOf('%s') > -1) {
				items[i].click();
				return JSON.stringify({success: true, matched: t});
			}
		}
		return JSON.stringify({success: false, reason: 'job not found'});
	})()`, strings.ReplaceAll(jobName, "'", "\\'"))

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
	// Format: "职位名 _ 地点 薪资" or "职位名（关闭） _ 地点 薪资"
	closed := strings.Contains(text, "（关闭）") || strings.Contains(text, "(关闭)")
	name := text
	var location, salary string

	parts := strings.Split(text, " _ ")
	if len(parts) >= 2 {
		name = strings.TrimSpace(parts[0])
		rest := strings.TrimSpace(parts[1])
		// rest might be "北京 20-40K" or "北京 400-500元/天"
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
