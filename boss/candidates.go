package boss

import (
	"github.com/Rhypoo-Ma/BOSS-cli/browser"
	"encoding/json"
	"fmt"
	"strings"
)

type Candidate struct {
	Name     string `json:"name"`
	Job      string `json:"job,omitempty"`
	Status   string `json:"status,omitempty"`
	LastMsg  string `json:"last_msg,omitempty"`
	Time     string `json:"time,omitempty"`
}

func ListCandidates(client *browser.Client, filterStatus string) ([]Candidate, error) {
	// If filterStatus provided, click the corresponding tab first
	if filterStatus != "" {
		if err := clickFilterTab(client, filterStatus); err != nil {
			return nil, err
		}
	}

	code := `(function(){
		var items = document.querySelectorAll('.geek-item-wrap');
		var result = [];
		for (var i = 0; i < items.length; i++) {
			var lines = items[i].innerText.split('\n').map(function(s){ return s.trim(); }).filter(function(s){ return s.length > 0; });
			result.push(lines.slice(0, 6));
		}
		return JSON.stringify(result.slice(0, 50));
	})()`

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return nil, fmt.Errorf("evaluate failed: %w", err)
	}

	var items [][]string
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	candidates := parseCandidates(items)
	return candidates, nil
}

func clickFilterTab(client *browser.Client, status string) error {
	code := fmt.Sprintf(`(function(){
		var els = document.querySelectorAll('div, span, a, li');
		for (var i = 0; i < els.length; i++) {
			var t = els[i].textContent.trim();
			if (t.indexOf('%s') > -1 && t.length < 20) {
				els[i].click();
				return JSON.stringify({success: true});
			}
		}
		return JSON.stringify({success: false});
	})()`, strings.ReplaceAll(status, "'", "\\'"))

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return fmt.Errorf("evaluate failed: %w", err)
	}
	var result struct {
		Success bool `json:"success"`
	}
	json.Unmarshal(raw, &result)
	return nil
}

func parseCandidates(items [][]string) []Candidate {
	var candidates []Candidate
	for _, lines := range items {
		cand := parseCandidateLines(lines)
		if cand.Name != "" {
			candidates = append(candidates, cand)
		}
	}
	return candidates
}

func parseCandidateLines(lines []string) Candidate {
	var cand Candidate
	// BOSS chat list format (variations observed):
	// [count, time, name, job, lastMsg] or [time, name, job, lastMsg]
	// Skip count lines (pure numbers like "1", "2")
	var filtered []string
	for _, line := range lines {
		if line == "" || isNoiseLine(line) {
			continue
		}
		filtered = append(filtered, line)
	}

	for i, line := range filtered {
		if isTimeLine(line) && i+1 < len(filtered) {
			cand.Time = line
			cand.Name = filtered[i+1]
			if i+2 < len(filtered) {
				cand.Job = filtered[i+2]
			}
			if i+3 < len(filtered) {
				cand.LastMsg = filtered[i+3]
			}
			break
		}
	}
	return cand
}

func isTimeLine(s string) bool {
	// Matches patterns like "19:45", "04-17 13:12", "昨天 17:13"
	return strings.Contains(s, ":") && len(s) <= 12
}

func isNoiseLine(s string) bool {
	// Skip pure number counts and navigation labels
	if s == "" {
		return true
	}
	noise := []string{"全部", "新招呼", "沟通中", "已约面", "已获取简历", "已交换电话",
		"已交换微信", "收藏", "更多", "买赠", "帮你问牛人", "不符牛人",
		"全部职位", "未读", "批量", "职位管理", "推荐牛人", "搜索", "api",
		"沟通", "意向沟通", "大厂", "互动", "牛人管理", "道具", "首充礼",
		"工具箱", "招聘规范", "我的客服", "面试", "招聘数据", "账号权益",
	}
	for _, n := range noise {
		if s == n {
			return true
		}
	}
	return false
}
