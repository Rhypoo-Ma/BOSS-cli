package boss

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Rhypoo-Ma/BOSS-cli/browser"
)

type FilterResult struct {
	Job       string `json:"job"`
	Filter    string `json:"filter"`
	Unread    bool   `json:"unread"`
	Confirmed bool   `json:"confirmed"`
}

// SwitchJobWithFilters switches the page to the target state across three dimensions:
//   1. Job (岗位)
//   2. Communication status filter (沟通状态)
//   3. Message status filter (消息状态: 全部 / 未读)
//
// Each dimension is switched to the requested target and verified independently with retries.
func SwitchJobWithFilters(client *browser.Client, jobName, filterStatus string, unreadOnly bool) (*FilterResult, error) {
	var result FilterResult

	// Normalize inputs
	if filterStatus == "" {
		filterStatus = "全部"
	}

	// Dimension 1: job
	if err := SwitchJob(client, jobName); err != nil {
		return nil, fmt.Errorf("switch job failed: %w", err)
	}

	// Dimension 2: communication status filter (always enforce target state)
	if err := clickFilterTab(client, filterStatus); err != nil {
		return nil, fmt.Errorf("click filter tab failed: %w", err)
	}

	// Dimension 3: message status filter (always enforce target state)
	targetMsg := "全部"
	if unreadOnly {
		targetMsg = "未读"
	}
	if err := setMessageFilter(client, targetMsg); err != nil {
		return nil, fmt.Errorf("set message filter failed: %w", err)
	}

	// Final verification with retries
	err := browser.Retry(func() error {
		var verifyErr error
		result, verifyErr = verifyFilters(client, jobName, filterStatus, unreadOnly)
		if verifyErr != nil {
			return verifyErr
		}
		if !result.Confirmed {
			return fmt.Errorf("verification mismatch: job=%q filter=%q unread=%v", result.Job, result.Filter, result.Unread)
		}
		return nil
	}, 3, 1*time.Second)

	if err != nil {
		return &result, err
	}
	return &result, nil
}

// filterTabSelector returns a JavaScript expression that matches communication status tabs.
func filterTabSelector() string {
	return `document.querySelectorAll('.chat-label-item, .filter-tab, [class*="label-item"], [class*="filter-item"], .chat-tabs span, .chat-tabs div')`
}

func clickFilterTab(client *browser.Client, status string) error {
	return browser.Retry(func() error {
		return clickFilterTabOnce(client, status)
	}, 3, 800*time.Millisecond)
}

func clickFilterTabOnce(client *browser.Client, status string) error {
	// If already on target tab, nothing to do
	if isFilterActive(client, status) {
		return nil
	}

	code := fmt.Sprintf(`(function(){
		var tabs = %s;
		for (var i = 0; i < tabs.length; i++) {
			var t = tabs[i].textContent.trim();
			// Match exact text or text with count suffix like "新招呼(12)"
			if (t === '%s' || t.indexOf('%s') === 0 || t.indexOf('%s（') === 0 || t.indexOf('%s(') === 0) {
				tabs[i].click();
				return JSON.stringify({success: true, matched: t});
			}
		}
		return JSON.stringify({success: false, reason: 'filter tab not found'});
	})()`, filterTabSelector(), strings.ReplaceAll(status, "'", "\\'"), strings.ReplaceAll(status, "'", "\\'"), strings.ReplaceAll(status, "'", "\\'"), strings.ReplaceAll(status, "'", "\\'"))

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
		return fmt.Errorf("%s", result.Reason)
	}

	// Wait for the clicked tab to become active
	if err := client.WaitFor(func() string {
		return fmt.Sprintf(`%s`, isFilterActiveCondition(status))
	}(), 3*time.Second, 200*time.Millisecond); err != nil {
		return fmt.Errorf("filter tab %q did not become active: %w", status, err)
	}
	return nil
}

func isFilterActive(client *browser.Client, status string) bool {
	code := fmt.Sprintf(`(function(){
		return JSON.stringify({active: %s});
	})()`, isFilterActiveCondition(status))
	raw, err := client.EvaluateValue(code)
	if err != nil {
		return false
	}
	var r struct {
		Active bool `json:"active"`
	}
	json.Unmarshal(raw, &r)
	return r.Active
}

func isFilterActiveCondition(status string) string {
	escapedStatus := strings.ReplaceAll(status, "'", "\\'")
	return fmt.Sprintf(`(function(){
		var tabs = %s;
		for (var i = 0; i < tabs.length; i++) {
			var t = tabs[i].textContent.trim();
			if (t === '%s' || t.indexOf('%s') === 0 || t.indexOf('%s（') === 0 || t.indexOf('%s(') === 0) {
				var cls = tabs[i].className || '';
				return cls.indexOf('active') > -1 || cls.indexOf('selected') > -1 || cls.indexOf('cur') > -1 || tabs[i].getAttribute('aria-selected') === 'true';
			}
		}
		return false;
	})()`, filterTabSelector(), escapedStatus, escapedStatus, escapedStatus, escapedStatus)
}

// messageFilterContainerSelector returns a JS expression for the message filter container.
func messageFilterContainerSelector() string {
	return `document.querySelector('.chat-message-filter-left, [class*="filter-left"]')`
}

func setMessageFilter(client *browser.Client, target string) error {
	return browser.Retry(func() error {
		return setMessageFilterOnce(client, target)
	}, 3, 800*time.Millisecond)
}

func setMessageFilterOnce(client *browser.Client, target string) error {
	// If already on target, nothing to do
	if isMessageFilterActive(client, target) {
		return nil
	}

	code := fmt.Sprintf(`(function(){
		var container = %s;
		if (!container) return JSON.stringify({success: false, reason: 'message filter container not found'});
		var items = container.querySelectorAll('span, div, label');
		for (var i = 0; i < items.length; i++) {
			var t = items[i].textContent.trim();
			if (t === '%s') {
				items[i].click();
				return JSON.stringify({success: true});
			}
		}
		return JSON.stringify({success: false, reason: '%s not found'});
	})()`, messageFilterContainerSelector(), strings.ReplaceAll(target, "'", "\\'"), strings.ReplaceAll(target, "'", "\\'"))

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return fmt.Errorf("evaluate failed: %w", err)
	}
	var result struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("%s", result.Reason)
	}

	// Wait for target to become active
	if err := client.WaitFor(isMessageFilterActiveCondition(target), 3*time.Second, 200*time.Millisecond); err != nil {
		return fmt.Errorf("message filter %q did not become active: %w", target, err)
	}
	return nil
}

func isMessageFilterActive(client *browser.Client, target string) bool {
	code := fmt.Sprintf(`(function(){
		return JSON.stringify({active: %s});
	})()`, isMessageFilterActiveCondition(target))
	raw, err := client.EvaluateValue(code)
	if err != nil {
		return false
	}
	var r struct {
		Active bool `json:"active"`
	}
	json.Unmarshal(raw, &r)
	return r.Active
}

func isMessageFilterActiveCondition(target string) string {
	escapedTarget := strings.ReplaceAll(target, "'", "\\'")
	return fmt.Sprintf(`(function(){
		var container = %s;
		if (!container) return false;
		var items = container.querySelectorAll('span, div, label');
		for (var j = 0; j < items.length; j++) {
			var t = items[j].textContent.trim();
			if (t === '%s') {
				var cls = items[j].className || '';
				return cls.indexOf('active') > -1 || cls.indexOf('selected') > -1 || cls.indexOf('cur') > -1 || items[j].getAttribute('aria-selected') === 'true';
			}
		}
		return false;
	})()`, messageFilterContainerSelector(), escapedTarget)
}

func verifyFilters(client *browser.Client, expectedJob, expectedFilter string, expectedUnread bool) (FilterResult, error) {
	code := `(function(){
		var result = {job: '', filter: '', unread: false, confirmed: false};
		
		// Check job label (scoped inside .chat-top-job to avoid matching user menu)
		var jobEl = document.querySelector('.chat-top-job .chat-select-job');
		if (jobEl) result.job = jobEl.innerText.trim();
		
		// Check active communication status tab
		var tabs = document.querySelectorAll('.chat-label-item, .filter-tab, [class*="label-item"], [class*="filter-item"], .chat-tabs span, .chat-tabs div');
		for (var i = 0; i < tabs.length; i++) {
			var cls = tabs[i].className || '';
			var aria = tabs[i].getAttribute('aria-selected');
			if (cls.indexOf('selected') > -1 || cls.indexOf('active') > -1 || cls.indexOf('cur') > -1 || aria === 'true') {
				result.filter = tabs[i].innerText.trim();
				break;
			}
		}
		
		// Check unread filter
		var unreadContainer = document.querySelector('.chat-message-filter-left, [class*="filter-left"]');
		if (unreadContainer) {
			var items = unreadContainer.querySelectorAll('span, div, label');
			for (var j = 0; j < items.length; j++) {
				var t = items[j].innerText.trim();
				if (t === '未读') {
					var cls = items[j].className || '';
					var aria = items[j].getAttribute('aria-selected');
					if (cls.indexOf('active') > -1 || cls.indexOf('selected') > -1 || cls.indexOf('cur') > -1 || aria === 'true') {
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
		return FilterResult{}, err
	}
	var result FilterResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return FilterResult{}, err
	}

	expectedFilterText := expectedFilter
	if expectedFilterText == "" {
		expectedFilterText = "全部"
	}

	jobOK := strings.Contains(result.Job, expectedJob)
	filterOK := expectedFilterText == "全部" || strings.Contains(result.Filter, expectedFilterText)
	unreadOK := !expectedUnread || result.Unread

	result.Confirmed = jobOK && filterOK && unreadOK
	return result, nil
}
