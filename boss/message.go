package boss

import (
	"github.com/Rhypoo-Ma/BOSS-cli/browser"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func SendMessage(client *browser.Client, candidateName string, message string) error {
	// Step 1: click the candidate row
	code := fmt.Sprintf(`(function(){
		var items = document.querySelectorAll('.geek-item-wrap, .geek-item, .friend-item, .chat-item');
		for (var i = 0; i < items.length; i++) {
			if (items[i].textContent.indexOf('%s') > -1) {
				items[i].click();
				return JSON.stringify({success: true});
			}
		}
		return JSON.stringify({success: false, reason: 'candidate not found'});
	})()`, strings.ReplaceAll(candidateName, "'", "\\'"))

	raw, err := client.EvaluateValue(code)
	if err != nil {
		return fmt.Errorf("click candidate failed: %w", err)
	}
	var result struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse failed: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("open candidate failed: %s", result.Reason)
	}

	time.Sleep(2000 * time.Millisecond)

	// Step 2: click input, focus, fill via innerHTML and send via native click + Enter key
	safeMsg := strings.ReplaceAll(message, "'", "\\'")
	sendCode := fmt.Sprintf(`(function(){
		var input = document.querySelector('.boss-chat-editor-input');
		if (!input) return JSON.stringify({success: false, reason: 'input not found'});
		input.click();
		input.focus();
		input.innerHTML = '%s';
		input.dispatchEvent(new Event('input', {bubbles: true}));
		input.dispatchEvent(new Event('change', {bubbles: true}));
		// Trigger keyboard event to ensure button becomes active
		input.dispatchEvent(new KeyboardEvent('keydown', {key: 'End', bubbles: true}));
		input.dispatchEvent(new KeyboardEvent('keyup', {key: 'End', bubbles: true}));
		setTimeout(function(){
			var btn = document.querySelector('.conversation-editor .submit');
			if (btn) { btn.click(); }
		}, 300);
		return JSON.stringify({success: true});
	})()`, safeMsg)

	raw, err = client.EvaluateValue(sendCode)
	if err != nil {
		return fmt.Errorf("send message failed: %w", err)
	}
	// Wait for async send
	time.Sleep(1500 * time.Millisecond)
	// Double-check by clicking submit again if still present
	retryCode := `(function(){
		var btn = document.querySelector('.conversation-editor .submit');
		if (btn && btn.classList.contains('active')) { btn.click(); return JSON.stringify({retry: true}); }
		return JSON.stringify({retry: false});
	})()`
	client.EvaluateValue(retryCode)
	time.Sleep(500 * time.Millisecond)
	return nil
}
