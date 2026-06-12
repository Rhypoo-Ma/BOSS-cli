package boss

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Rhypoo-Ma/BOSS-cli/browser"
)

type Candidate struct {
	Name    string `json:"name"`
	Job     string `json:"job,omitempty"`
	Status  string `json:"status,omitempty"`
	LastMsg string `json:"last_msg,omitempty"`
	Time    string `json:"time,omitempty"`
}

type candidateKey struct {
	Name    string
	Time    string
	LastMsg string
}

func (c Candidate) key() candidateKey {
	return candidateKey{Name: c.Name, Time: c.Time, LastMsg: c.LastMsg}
}

// ListCandidates returns candidates from the current job/filter view.
// If all=true, it scrolls through the virtual list to collect more candidates.
// max limits the total number when all=true (0 means no limit).
func ListCandidates(client *browser.Client, filterStatus string, all bool, max int) ([]Candidate, error) {
	// If filterStatus provided, click the corresponding tab first
	if filterStatus != "" && filterStatus != "全部" {
		if err := clickFilterTab(client, filterStatus); err != nil {
			return nil, err
		}
	}

	if all {
		return listAllCandidates(client, max)
	}
	return listVisibleCandidates(client)
}

func listVisibleCandidates(client *browser.Client) ([]Candidate, error) {
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

	return parseCandidates(items), nil
}

func listAllCandidates(client *browser.Client, max int) ([]Candidate, error) {
	seen := make(map[candidateKey]bool)
	result := make([]Candidate, 0)

	scrollStep := 600
	maxIterations := 100
	stuckCount := 0
	const maxStuck = 10

	for iteration := 0; iteration < maxIterations; iteration++ {
		// Collect currently visible candidates
		visible, err := listVisibleCandidates(client)
		if err != nil {
			return nil, err
		}
		newCount := 0
		for _, c := range visible {
			if c.Name == "" {
				continue
			}
			k := c.key()
			if seen[k] {
				continue
			}
			seen[k] = true
			result = append(result, c)
			newCount++
		}

		// Check max limit
		if max > 0 && len(result) >= max {
			if len(result) > max {
				result = result[:max]
			}
			return result, nil
		}

		// Scroll the list
		atBottom, err := scrollList(client, scrollStep)
		if err != nil {
			return result, err
		}
		if atBottom {
			// One final collection after reaching bottom
			visible, err := listVisibleCandidates(client)
			if err == nil {
				for _, c := range visible {
					if c.Name == "" {
						continue
					}
					k := c.key()
					if seen[k] {
						continue
					}
					seen[k] = true
					result = append(result, c)
				}
			}
			if max > 0 && len(result) > max {
				result = result[:max]
			}
			return result, nil
		}

		// Safety valve: if no new candidates appear for too long, stop
		if newCount == 0 {
			stuckCount++
			if stuckCount >= maxStuck {
				return result, nil
			}
		} else {
			stuckCount = 0
		}

		// Wait for virtual list to render new items
		time.Sleep(300 * time.Millisecond)
	}

	return result, nil
}

// scrollList scrolls the candidate list down by step pixels and returns true if the bottom is reached.
func scrollList(client *browser.Client, step int) (bool, error) {
	code := fmt.Sprintf(`(function(){
		var list = document.querySelector('.user-list, .chat-list, [class*="user-list"], [class*="conversation-list"]');
		if (!list) return JSON.stringify({error: 'list container not found'});
		var beforeTop = list.scrollTop;
		list.scrollTop += %d;
		var afterTop = list.scrollTop;
		var maxScroll = list.scrollHeight - list.clientHeight;
		return JSON.stringify({
			beforeTop: beforeTop,
			afterTop: afterTop,
			maxScroll: maxScroll,
			atBottom: afterTop >= maxScroll - 5
		});
	})()`, step)

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return false, fmt.Errorf("scroll failed: %w", err)
	}
	var r struct {
		BeforeTop int    `json:"beforeTop"`
		AfterTop  int    `json:"afterTop"`
		MaxScroll int    `json:"maxScroll"`
		AtBottom  bool   `json:"atBottom"`
		Error     string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return false, fmt.Errorf("parse scroll result failed: %w", err)
	}
	if r.Error != "" {
		return false, fmt.Errorf(r.Error)
	}
	return r.AtBottom, nil
}

func parseCandidates(items [][]string) []Candidate {
	candidates := make([]Candidate, 0)
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
	// Matches patterns like "19:45", "04-17 13:12", "昨天", "06月10日", "刚刚"
	if s == "昨天" || s == "刚刚" || s == "今天" {
		return true
	}
	if strings.Contains(s, "月") && strings.Contains(s, "日") {
		return true
	}
	if strings.Contains(s, "-") && strings.Contains(s, ":") {
		return true
	}
	return strings.Contains(s, ":") && len(s) <= 12
}

func isNoiseLine(s string) bool {
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
