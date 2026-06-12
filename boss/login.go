package boss

import (
	"github.com/Rhypoo-Ma/BOSS-cli/browser"
	"encoding/json"
	"fmt"
	"time"
)

type LoginStatus struct {
	LoggedIn bool   `json:"loggedIn"`
	User     string `json:"user,omitempty"`
}

func CheckLogin(client *browser.Client) (*LoginStatus, error) {
	// First check if we are already on the chat page and logged in
	checkCode := `(function(){
		var href = location.href;
		var el = document.querySelector('.user-name, .boss-name, .name, .username, .nav-username');
		if (el) return JSON.stringify({loggedIn: true, user: el.textContent.trim(), href: href});
		var menu = document.querySelector('.menu-list, .side-wrap');
		var body = document.body.innerText;
		if (menu && menu.innerText.indexOf('沟通') > -1) {
			return JSON.stringify({loggedIn: true, user: '', href: href});
		}
		if (body.indexOf('职位管理') > -1 && body.indexOf('沟通') > -1 && body.indexOf('牛人管理') > -1) {
			return JSON.stringify({loggedIn: true, user: '', href: href});
		}
		return JSON.stringify({loggedIn: false, user: '', href: href});
	})()`

	raw, err := client.EvaluateValue(checkCode)
	if err != nil {
		return nil, fmt.Errorf("evaluate failed: %w", err)
	}
	var status LoginStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}
	if status.LoggedIn {
		return &status, nil
	}

	// Not logged in or not on chat page, navigate to chat page
	if err := client.Navigate("https://www.zhipin.com/web/chat/index"); err != nil {
		return nil, fmt.Errorf("navigate failed: %w", err)
	}

	code := `(function(){
		var el = document.querySelector('.user-name, .boss-name, .name, .username, .nav-username');
		if (el) return JSON.stringify({loggedIn: true, user: el.textContent.trim()});
		var menu = document.querySelector('.menu-list, .side-wrap');
		var body = document.body.innerText;
		if (menu && menu.innerText.indexOf('沟通') > -1) {
			return JSON.stringify({loggedIn: true, user: ''});
		}
		if (body.indexOf('职位管理') > -1 && body.indexOf('沟通') > -1 && body.indexOf('牛人管理') > -1) {
			return JSON.stringify({loggedIn: true, user: ''});
		}
		return JSON.stringify({loggedIn: false, user: ''});
	})()`

	for i := 0; i < 6; i++ {
		raw, err = client.EvaluateValue(code)
		if err != nil {
			return nil, fmt.Errorf("evaluate failed: %w", err)
		}
		if err := json.Unmarshal(raw, &status); err != nil {
			return nil, fmt.Errorf("parse failed: %w", err)
		}
		if status.LoggedIn {
			return &status, nil
		}
		time.Sleep(1 * time.Second)
	}

	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}
	// Wait for async content to load after fresh navigation
	time.Sleep(2 * time.Second)
	return &status, nil
}
