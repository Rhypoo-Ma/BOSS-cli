package browser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const DefaultDaemonURL = "http://127.0.0.1:10086"

type Client struct {
	baseURL string
	session string
	http    *http.Client
}

func NewClient(session string) *Client {
	return &Client{
		baseURL: DefaultDaemonURL,
		session: session,
		http:    &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *Client) Call(action string, args map[string]any) (json.RawMessage, error) {
	body, _ := json.Marshal(map[string]any{
		"action":  action,
		"session": c.session,
		"args":    args,
	})
	resp, err := c.http.Post(c.baseURL+"/command", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daemon unreachable: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
	}
	return result.Data, nil
}

func (c *Client) Navigate(url string) error {
	// Try reusing current tab first to preserve login session
	_, err := c.Call("navigate", map[string]any{"url": url, "newTab": false})
	if err != nil {
		// Fallback to new tab if current tab is unavailable
		_, err = c.Call("navigate", map[string]any{"url": url, "newTab": true})
	}
	return err
}

func (c *Client) Evaluate(code string) (json.RawMessage, error) {
	return c.Call("evaluate", map[string]any{"code": code})
}

// EvaluateValue returns the raw JSON value produced by the browser evaluate call.
// It unwraps the webbridge wrapper and, for string-typed results, decodes the JSON
// string so the caller receives valid JSON bytes.
func (c *Client) EvaluateValue(code string) (json.RawMessage, error) {
	raw, err := c.Evaluate(code)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.Type == "string" {
		var s string
		if err := json.Unmarshal(wrapper.Value, &s); err != nil {
			return nil, err
		}
		return json.RawMessage(s), nil
	}
	return wrapper.Value, nil
}

func (c *Client) Click(selector string) error {
	_, err := c.Call("click", map[string]any{"selector": selector})
	return err
}

func (c *Client) Snapshot() (json.RawMessage, error) {
	return c.Call("snapshot", map[string]any{})
}

func (c *Client) NetworkStart() error  { _, err := c.Call("network", map[string]any{"cmd": "start"}); return err }
func (c *Client) NetworkStop() error   { _, err := c.Call("network", map[string]any{"cmd": "stop"}); return err }
func (c *Client) NetworkList() (json.RawMessage, error) {
	return c.Call("network", map[string]any{"cmd": "list"})
}
