#!/usr/bin/env python3
"""
Example: Bulk screen candidates and send messages.

Prerequisites:
  1. Chrome + kimi-webbridge extension installed and running
  2. BOSS-cli Go binary compiled and in PATH (or run `go run .`)
  3. Logged in to https://www.zhipin.com/web/chat/index

Workflow:
  1. Use BOSS-cli to switch to the target job + filter + unread
  2. Run this script to scroll through candidates, screen by custom rules,
     and send messages to matches.

Customize:
  - TARGET_SCHOOLS: list of school keywords to match
  - TARGET_COMPANIES: list of company keywords to match in experience
  - MSG: message to send
  - get_school() / get_internship(): implement your own matching logic
"""

import json
import time
import urllib.request

DAEMON = "http://127.0.0.1:10086"
SESSION = "BOSS-cli"

# --- Customize these ---
TARGET_SCHOOLS = [
    "清华大学", "北京大学", "复旦大学", "上海交通大学", "浙江大学",
    # Add more schools...
]

TARGET_COMPANIES = [
    "字节", "字节跳动", "阿里", "阿里巴巴", "百度", "美团", "滴滴",
]

MSG = "你好，感谢投递！可以简单介绍一下你的项目经历吗？"
# -----------------------


def call(action, args):
    req = urllib.request.Request(
        f"{DAEMON}/command",
        data=json.dumps({"action": action, "session": SESSION, "args": args}).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        resp = urllib.request.urlopen(req, timeout=30)
        return json.loads(resp.read().decode())
    except Exception as e:
        return {"ok": False, "error": str(e)}


def evaluate(code):
    r = call("evaluate", {"code": code})
    if not r.get("ok"):
        return None, r.get("error")
    d = r.get("data", {})
    if d.get("type") == "string":
        try:
            return json.loads(d["value"]), None
        except:
            return d["value"], None
    return d, None


def get_visible_candidates():
    """Return list of visible candidate DOM elements as text arrays."""
    code = """(function(){
        var items = document.querySelectorAll('.geek-item-wrap');
        var result = [];
        for (var i = 0; i < items.length; i++) {
            var lines = items[i].innerText.split('\n')
                .map(function(s){ return s.trim(); })
                .filter(function(s){ return s.length > 0; });
            result.push(lines);
        }
        return JSON.stringify(result);
    })()"""
    return evaluate(code)


def click_candidate_by_name(name):
    safe = name.replace("\\", "\\\\").replace("'", "\\'")
    code = f"""(function(){{
        var items = document.querySelectorAll('.geek-item-wrap');
        for (var i = 0; i < items.length; i++) {{
            if (items[i].textContent.indexOf('{safe}') > -1) {{
                items[i].click();
                return JSON.stringify({{success: true}});
            }}
        }}
        return JSON.stringify({{success: false}});
    }})()"""
    return evaluate(code)


def get_chat_detail():
    """Extract education & experience text from open chat panel."""
    code = """(function(){
        var detail = document.querySelector('.chat-conversation-detail, .geek-detail-panel, .detail-panel');
        if (!detail) return JSON.stringify({texts: []});
        var els = detail.querySelectorAll('div, span, p, li');
        var texts = [];
        for (var i = 0; i < els.length; i++) {
            var t = els[i].innerText.trim();
            if (t.length > 0) texts.push(t);
        }
        return JSON.stringify({texts: texts});
    })()"""
    return evaluate(code)


def send_message(msg):
    safe = msg.replace("\\", "\\\\").replace("'", "\\'")
    code = f"""(function(){{
        var input = document.querySelector('.boss-chat-editor-input');
        if (!input) return JSON.stringify({{success: false, reason: 'no input'}});
        input.click();
        input.focus();
        input.innerHTML = '{safe}';
        input.dispatchEvent(new Event('input', {{bubbles: true}}));
        input.dispatchEvent(new Event('change', {{bubbles: true}}));
        input.dispatchEvent(new KeyboardEvent('keydown', {{key: 'End', bubbles: true}}));
        input.dispatchEvent(new KeyboardEvent('keyup', {{key: 'End', bubbles: true}}));
        setTimeout(function(){{
            var btn = document.querySelector('.conversation-editor .submit');
            if (btn) {{ btn.click(); }}
        }}, 300);
        return JSON.stringify({{success: true}});
    }})()"""
    return evaluate(code)


def verify_sent(msg_keyword):
    """Check last few chat messages contain the keyword."""
    safe = msg_keyword.replace("\\", "\\\\").replace("'", "\\'")
    code = f"""(function(){{
        var msgs = document.querySelectorAll('.message-item, .msg-content');
        for (var i = Math.max(0, msgs.length - 4); i < msgs.length; i++) {{
            if (msgs[i].innerText.indexOf('{safe}') > -1)
                return JSON.stringify({{found: true}});
        }}
        return JSON.stringify({{found: false}});
    }})()"""
    return evaluate(code)


def scroll_list(pixels=400):
    code = f"""(function(){{
        var list = document.querySelector('.user-list');
        if (list) {{ list.scrollTop += {pixels}; return JSON.stringify({{scrolled: true}}); }}
        return JSON.stringify({{scrolled: false}});
    }})()"""
    return evaluate(code)


# --- Matching logic (customize as needed) ---

def match_school(texts):
    full = "\n".join(texts)
    for s in TARGET_SCHOOLS:
        if s in full:
            return s
    return None


def match_company(texts):
    full = "\n".join(texts)
    for c in TARGET_COMPANIES:
        if c in full:
            return c
    return None


# --- Main workflow ---

def main():
    print("=" * 50)
    print("BOSS Bulk Screen & Send Example")
    print("=" * 50)
    print()
    print("IMPORTANT: Before running this script:")
    print("  1. Ensure kimi-webbridge daemon is running")
    print("  2. Log in to BOSS Zhipin in Chrome")
    print("  3. Run: BOSS-cli switch-job '岗位名' --filter='新招呼' --unread")
    print()
    input("Press Enter to start screening current visible batch...")

    data, err = get_visible_candidates()
    if err or not data:
        print(f"Failed to get candidates: {err}")
        return

    candidates = data if isinstance(data, list) else []
    print(f"Found {len(candidates)} visible candidates")

    for cand_texts in candidates:
        name = cand_texts[0] if cand_texts else "Unknown"
        print(f"\n--- Checking: {name} ---")

        # Click to open detail
        click_candidate_by_name(name)
        time.sleep(1.5)

        detail, _ = get_chat_detail()
        texts = detail.get("texts", []) if detail else []

        school = match_school(texts)
        company = match_company(texts)

        print(f"  School match: {school or 'NONE'}")
        print(f"  Company match: {company or 'NONE'}")

        if school and company:
            print(f"  >>> MATCH! Sending message...")
            send_message(MSG)
            time.sleep(1.5)
            ok, _ = verify_sent(MSG[:10])
            if ok and ok.get("found"):
                print(f"  >>> [送达] Verified sent.")
            else:
                print(f"  >>> [?] Send verification inconclusive.")
        else:
            print(f"  >>> SKIP (does not match criteria)")

    print("\nBatch complete.")


if __name__ == "__main__":
    main()
