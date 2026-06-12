package boss

import (
	"github.com/Rhypoo-Ma/BOSS-cli/browser"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type FilterResult struct {
	Job       string `json:"job"`
	Filter    string `json:"filter"`
	Unread    bool   `json:"unread"`
	Confirmed bool   `json:"confirmed"`
}

// SwitchJobWithFilters switches job and optionally applies status filter and unread filter.
// It verifies all three dimensions before returning.
func SwitchJobWithFilters(client *browser.Client, jobName, filterStatus string, unreadOnly bool) (*FilterResult, error) {
	// Step 1: switch job
	if err := SwitchJob(client, jobName); err != nil {
		return nil, fmt.Errorf("switch job failed: %w", err)
	}
	time.Sleep(2 * time.Second)

	// Step 2: apply communication status filter (if not "全部")
	if filterStatus != "" && filterStatus != "全部" {
		if err := clickFilterTab(client, filterStatus); err != nil {
			return nil, fmt.Errorf("click filter tab failed: %w", err)
		}
		time.Sleep(1500 * time.Millisecond)
	}

	// Step 3: apply unread filter (if requested)
	if unreadOnly {
		if err := clickUnreadFilter(client); err != nil {
			return nil, fmt.Errorf("click unread filter failed: %w", err)
		}
		time.Sleep(1500 * time.Millisecond)
	}

	// Step 4: verify current state
	result, err := verifyFilters(client, jobName, filterStatus, unreadOnly)
	if err != nil {
		return nil, fmt.Errorf("verify filters failed: %w", err)
	}
	if !result.Confirmed {
		return result, fmt.Errorf("filter verification failed: job=%s filter=%s unread=%v", result.Job, result.Filter, result.Unread)
	}
	return result, nil
}

func clickUnreadFilter(client *browser.Client) error {
	code := `(function(){
		var container = document.querySelector('.chat-message-filter-left');
		if (!container) return JSON.stringify({success: false, reason: 'filter container not found'});
		var items = container.querySelectorAll('span, div');
		for (var i = 0; i < items.length; i++) {
			var t = items[i].innerText;
			if (t && t.trim() === '未读') {
				items[i].click();
				return JSON.stringify({success: true});
			}
		}
		return JSON.stringify({success: false, reason: '未读 not found'});
	})()`

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return err
	}
	var result struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return err
	}
	if !result.Success {
		return fmt.Errorf("%s", result.Reason)
	}
	return nil
}

func verifyFilters(client *browser.Client, expectedJob, expectedFilter string, expectedUnread bool) (*FilterResult, error) {
	code := `(function(){
		var result = {job: '', filter: '', unread: false, confirmed: false};
		
		// Check job
		var jobEl = document.querySelector('.chat-select-job');
		if (jobEl) result.job = jobEl.innerText.trim();
		
		// Check active filter tab
		var tabs = document.querySelectorAll('.chat-label-item, .filter-tab, [class*="label-item"]');
		for (var i = 0; i < tabs.length; i++) {
			var className = tabs[i].className || '';
			if (className.indexOf('selected') > -1 || className.indexOf('active') > -1) {
				result.filter = tabs[i].innerText.trim();
				break;
			}
		}
		
		// Check unread filter
		var unreadContainer = document.querySelector('.chat-message-filter-left');
		if (unreadContainer) {
			var items = unreadContainer.querySelectorAll('span, div');
			for (var j = 0; j < items.length; j++) {
				var t = items[j].innerText;
				if (t && t.trim() === '未读') {
					var cls = items[j].className || '';
					if (cls.indexOf('active') > -1 || cls.indexOf('selected') > -1) {
						result.unread = true;
					}
					break;
				}
			}
		}
		
		return JSON.stringify(result);
	})()`

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return nil, err
	}
	var result FilterResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	// Determine expected filter text (with possible count suffix)
	var expectedFilterText string
	if expectedFilter == "" || expectedFilter == "全部" {
		expectedFilterText = "全部"
	} else {
		expectedFilterText = expectedFilter
	}

	// Check job match (partial match allowed since job text includes location/salary)
	jobOK := strings.Contains(result.Job, expectedJob)

	// Check filter match (partial match since filter text includes count like "新招呼(123)")
	filterOK := expectedFilterText == "全部" || strings.Contains(result.Filter, expectedFilterText)

	// Check unread match
	unreadOK := !expectedUnread || result.Unread

	result.Confirmed = jobOK && filterOK && unreadOK
	return &result, nil
}
