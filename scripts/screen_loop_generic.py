#!/usr/bin/env python3
"""
Generic BOSS Zhipin auto-screening loop script.

Usage:
    1. Edit screen_loop_config.json (or the DEFAULT_CONFIG below) to set:
       - target_job: 岗位名称
       - filter_status: 筛选标签，如"新招呼"
       - unread_only: 是否只处理未读
       - target_schools: 学校关键词列表
       - target_companies: 公司关键词列表
       - enable_ai_keyword: 是否额外匹配含"AI"字眼的公司
       - message: 要发送的消息内容
    2. Run: python3 screen_loop_generic.py

Features:
    - Auto-switches job + filter + unread via BOSS-cli
    - Scrolls virtual list to collect visible candidates
    - Clicks each candidate, extracts education & experience text
    - Sends message if BOTH school AND company rules match
    - Maintains a sent-log to avoid duplicates across runs
    - Loops until no new unread candidates remain
"""

import json
import os
import re
import subprocess
import sys
import time
import urllib.request

DAEMON = "http://127.0.0.1:10086"
SESSION = "BOSS-cli"

# ---------------------------------------------------------------------------
# Default config (used when screen_loop_config.json does not exist)
# ---------------------------------------------------------------------------
DEFAULT_CONFIG = {
    "target_job": "商业分析实习生",
    "filter_status": "新招呼",
    "unread_only": True,
    "target_schools": [
        # 国内清北人复交浙江
        "清华大学", "清华",
        "北京大学", "北大",
        "中国人民大学", "人大",
        "复旦大学", "复旦",
        "上海交通大学", "上海交大", "上交",
        "浙江大学", "浙大",
        # QS前50 示例
        "麻省理工学院", "MIT",
        "帝国理工学院", "帝国理工",
        "牛津大学", "牛津",
        "剑桥大学", "剑桥",
        "哈佛大学", "哈佛",
        "斯坦福大学", "斯坦福",
        "新加坡国立大学", "新加坡国立", "NUS",
        "南洋理工大学", "南洋理工", "NTU",
        "香港大学", "港大", "HKU",
        "香港中文大学", "港中文", "CUHK",
        "香港科技大学", "港科大", "HKUST",
        "哥伦比亚大学", "哥伦比亚",
        "康奈尔大学", "康奈尔",
        "芝加哥大学", "芝加哥",
        "宾夕法尼亚大学", "宾大",
        "加州大学伯克利分校", "伯克利", "UC Berkeley",
        "墨尔本大学", "墨尔本",
        "悉尼大学", "悉尼",
        "多伦多大学", "多伦多",
        "伦敦大学学院", "UCL",
        "爱丁堡大学", "爱丁堡",
        "曼彻斯特大学", "曼彻斯特",
        "卡内基梅隆大学", "卡内基梅隆", "CMU",
        "杜克大学", "杜克",
        "西北大学", "Northwestern",
        "密歇根大学", "Michigan",
        "约翰霍普金斯大学", "Johns Hopkins",
        "布朗大学", "布朗",
        "波士顿大学", "Boston",
    ],
    "target_companies": [
        "字节", "字节跳动",
        "阿里", "阿里巴巴", "淘宝", "天猫", "支付宝", "阿里云", "蚂蚁",
        "美团",
        "滴滴", "滴滴出行",
        "快手",
        "小红书",
    ],
    "enable_ai_keyword": True,
    "message": "你认为互联网产品和ai产品的区别是"
}

CONFIG_FILE = os.path.join(os.path.dirname(__file__), "screen_loop_config.json")
SENT_LOG_FILE = os.path.join(os.path.dirname(__file__), "screen_loop_sent.json")


def load_config():
    if os.path.exists(CONFIG_FILE):
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            cfg = json.load(f)
        print(f"[INFO] Loaded config from {CONFIG_FILE}")
        return cfg
    print(f"[INFO] Config file not found, using DEFAULT_CONFIG.")
    return DEFAULT_CONFIG


def save_config(cfg):
    with open(CONFIG_FILE, "w", encoding="utf-8") as f:
        json.dump(cfg, f, ensure_ascii=False, indent=2)
    print(f"[INFO] Saved config to {CONFIG_FILE}")


def load_sent():
    if os.path.exists(SENT_LOG_FILE):
        with open(SENT_LOG_FILE, "r", encoding="utf-8") as f:
            return set(json.load(f))
    return set()


def save_sent(sent_set):
    with open(SENT_LOG_FILE, "w", encoding="utf-8") as f:
        json.dump(sorted(list(sent_set)), f, ensure_ascii=False, indent=2)


# ---------------------------------------------------------------------------
# Daemon helpers
# ---------------------------------------------------------------------------
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
        except Exception:
            return d["value"], None
    return d, None


# ---------------------------------------------------------------------------
# BOSS-cli helpers
# ---------------------------------------------------------------------------
def switch_job(job_name, filter_status, unread_only):
    cmd = ["./BOSS-cli", "switch-job", job_name, "--filter=" + filter_status]
    if unread_only:
        cmd.append("--unread")
    result = subprocess.run(cmd, capture_output=True, text=True, cwd=os.path.dirname(__file__))
    print("Switch job stdout:", result.stdout.strip())
    if result.returncode != 0:
        print("Switch job stderr:", result.stderr)
        return False
    # If unread_only is False, explicitly turn OFF the unread filter in case it was left on
    if not unread_only:
        code = """(function(){
            var container = document.querySelector('.chat-message-filter-left');
            if (!container) return JSON.stringify({success: false});
            var items = container.querySelectorAll('span, div');
            for (var i = 0; i < items.length; i++) {
                if (items[i].innerText.trim() === '未读') {
                    var cls = items[i].className || '';
                    if (cls.indexOf('active') > -1 || cls.indexOf('selected') > -1) {
                        items[i].click();
                        return JSON.stringify({success: true, action: 'turned_off'});
                    }
                    return JSON.stringify({success: true, action: 'already_off'});
                }
            }
            return JSON.stringify({success: false, reason: '未读 not found'});
        })()"""
        r, err = evaluate(code)
        print("Unread filter adjustment:", r, err)
        time.sleep(2)
    return True


# ---------------------------------------------------------------------------
# Candidate collection (handles virtual scrolling)
# ---------------------------------------------------------------------------
def collect_candidates():
    evaluate(
        "(function(){ var list=document.querySelector('.user-list'); if(list) list.scrollTop=0; return JSON.stringify({done:true}); })()"
    )
    time.sleep(1.5)

    all_names = set()
    all_candidates = []
    scroll_rounds = 0

    while scroll_rounds < 20:
        code = """(function(){
            var items = document.querySelectorAll('.geek-item-wrap');
            var result = [];
            for (var i = 0; i < items.length; i++) {
                var el = items[i];
                var text = el.innerText.split('\\n').map(function(s){return s.trim();}).filter(function(s){return s.length>0;});
                if (text.length < 3) continue;
                var nameEl = el.querySelector('.geek-name, .name, [class*=\\"geek-name\\"]');
                var name = nameEl ? nameEl.innerText.trim() : null;
                if (!name) {
                    var isNum = !isNaN(parseInt(text[0])) && text[0].length <= 3;
                    name = isNum && text.length >= 3 ? text[2] : text[1];
                }
                if (!name || name.match(/^\\d{2}:\\d{2}$/) || name.match(/^\\d{2}月\\d{2}日$/)) continue;
                result.push(name);
            }
            return JSON.stringify(result);
        })()"""
        r, _ = evaluate(code)
        if not r:
            r = []
        new_found = 0
        for name in r:
            if name not in all_names:
                all_names.add(name)
                all_candidates.append(name)
                new_found += 1
        print(f"  Scroll {scroll_rounds + 1}: visible={len(r)}, new={new_found}, total={len(all_candidates)}")
        if new_found == 0:
            break
        evaluate(
            "(function(){ var list=document.querySelector('.user-list'); if(list) list.scrollTop += 120; return JSON.stringify({done:true}); })()"
        )
        time.sleep(1)
        scroll_rounds += 1

    return all_candidates


# ---------------------------------------------------------------------------
# Candidate interaction
# ---------------------------------------------------------------------------
def scroll_to_candidate(name):
    code = (
        f"(function(){{ var items=document.querySelectorAll('.geek-item-wrap');"
        f" var list=document.querySelector('.user-list');"
        f" for(var i=0;i<items.length;i++){{"
        f" if(items[i].textContent.indexOf('{name}')>-1){{"
        f" if(list){{ list.scrollTop=items[i].offsetTop-100; }}"
        f" return JSON.stringify({{success:true}});"
        f" }} }} return JSON.stringify({{success:false}}); }})()"
    )
    return evaluate(code)


def click_candidate(name):
    code = (
        f"(function(){{ var items=document.querySelectorAll('.geek-item-wrap');"
        f" for(var i=0;i<items.length;i++){{"
        f" if(items[i].textContent.indexOf('{name}')>-1){{"
        f" items[i].click(); return JSON.stringify({{success:true}});"
        f" }} }} return JSON.stringify({{success:false}}); }})()"
    )
    return evaluate(code)


def get_panels():
    code = """(function(){
        var panels=document.querySelectorAll('.base-info-single-container, .base-info-single-main, .experience-content');
        var texts=[];
        for(var i=0;i<panels.length;i++){
            var t=panels[i].innerText;
            if(t && t.length>20){ texts.push(t); }
        }
        return JSON.stringify(texts);
    })()"""
    r, _ = evaluate(code)
    return r or []


def send_message(msg):
    safe = msg.replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n")
    code = f"""(function(){{
        var input = document.querySelector(".boss-chat-editor-input");
        if (!input) return JSON.stringify({{error:"no input"}});
        input.focus();
        input.innerText = "{safe}";
        input.dispatchEvent(new Event("input", {{bubbles:true}}));
        input.dispatchEvent(new Event("change", {{bubbles:true}}));
        setTimeout(function(){{
            var btn = document.querySelector(".conversation-editor .submit");
            if (btn) {{ btn.click(); }}
        }}, 300);
        return JSON.stringify({{success:true}});
    }})()"""
    return evaluate(code)


def verify_sent(msg_text):
    code = "(function(){ var msgs=document.querySelectorAll('.message-item, .message-bubble, .chat-message'); var result=[]; for(var i=Math.max(0,msgs.length-3);i<msgs.length;i++){ result.push(msgs[i].innerText); } return JSON.stringify(result); })()"
    r, _ = evaluate(code)
    if not r:
        return False
    return any(msg_text in str(m) for m in r)


# ---------------------------------------------------------------------------
# Screening logic
# ---------------------------------------------------------------------------
def check_school(full_text, schools):
    for school in schools:
        if school in full_text:
            return school
    return None


def check_company(full_text, companies, enable_ai):
    matched = []
    for company in companies:
        if company in full_text:
            matched.append(company)
    has_ai = False
    if enable_ai:
        has_ai = bool(re.search(r'\bAI\b', full_text, re.IGNORECASE))
    return matched, has_ai


def check_grade(full_text, grades):
    if not grades:
        return True, None
    for grade in grades:
        if grade in full_text:
            return True, grade
    return False, None


def process_candidates(candidates, sent_set, cfg):
    results = {}
    sent_in_round = []

    schools = cfg["target_schools"]
    companies = cfg["target_companies"]
    enable_ai = cfg.get("enable_ai_keyword", True)
    msg = cfg["message"]

    # Sort schools by length descending to prefer longer matches
    schools = sorted(schools, key=len, reverse=True)

    for i, name in enumerate(candidates, 1):
        if name in sent_set:
            print(f"\n[{i}/{len(candidates)}] {name} - SKIP (already sent)")
            results[name] = "skipped"
            continue

        print(f"\n[{i}/{len(candidates)}] Processing: {name}")

        r, _ = scroll_to_candidate(name)
        if r and r.get("success"):
            time.sleep(0.5)

        r, _ = click_candidate(name)
        if not r or not r.get("success"):
            print("  Cannot open chat")
            results[name] = "not_found"
            continue

        time.sleep(1.5)
        panels = get_panels()
        if not panels:
            print("  No info found")
            results[name] = "no_info"
            continue

        full_text = "\n".join(panels)

        has_school_check = bool(schools)
        matched_school = check_school(full_text, schools) if has_school_check else None
        matched_companies, has_ai = check_company(full_text, companies, enable_ai)
        has_target_company = len(matched_companies) > 0 or has_ai
        grade_ok, matched_grade = check_grade(full_text, cfg.get("target_grades", []))

        school_display = matched_school if has_school_check else "(unlimited)"
        grade_display = matched_grade if matched_grade else "(unlimited)"
        print(f"  School={school_display}, Companies={matched_companies}, AI_keyword={has_ai}, Grade={grade_display}")

        if has_school_check and not matched_school:
            print("  -> School NOT match, skip")
            results[name] = "school_no_match"
            continue

        if not has_target_company:
            print("  -> Company/AI NOT match, skip")
            results[name] = "work_no_match"
            continue

        if not grade_ok:
            print("  -> Grade NOT match, skip")
            results[name] = "grade_no_match"
            continue

        work_desc = matched_companies or ("AI_keyword" if has_ai else "")
        print(f"  -> MATCH! school={matched_school}, work={work_desc}")

        r, _ = send_message(msg)
        time.sleep(2)

        if verify_sent(msg):
            print("  -> Message confirmed sent")
            results[name] = "sent"
            sent_set.add(name)
            sent_in_round.append(name)
        else:
            print("  -> Message NOT found in chat")
            results[name] = "failed"
        time.sleep(1)

    return results, sent_in_round


# ---------------------------------------------------------------------------
# Main loop
# ---------------------------------------------------------------------------
def main():
    cfg = load_config()
    # Always persist config so user can edit the JSON file next time
    if not os.path.exists(CONFIG_FILE):
        save_config(cfg)

    print("=" * 60)
    print("Generic BOSS Auto-Screening Loop")
    print("=" * 60)
    print(f"Job        : {cfg['target_job']}")
    print(f"Filter     : {cfg['filter_status']}")
    print(f"Unread only: {cfg.get('unread_only', True)}")
    print(f"Schools    : {len(cfg['target_schools'])} keywords")
    print(f"Companies  : {len(cfg['target_companies'])} keywords")
    print(f"AI keyword : {cfg.get('enable_ai_keyword', True)}")
    print(f"Grades     : {cfg.get('target_grades', [])}")
    print(f"Message    : {cfg['message']}")
    print("=" * 60)

    if not switch_job(cfg["target_job"], cfg["filter_status"], cfg.get("unread_only", True)):
        sys.exit(1)
    time.sleep(2)

    sent_set = load_sent()
    total_sent = []
    round_num = 0
    max_rounds = 50

    while round_num < max_rounds:
        round_num += 1
        print(f"\n{'=' * 60}")
        print(f"ROUND {round_num}")
        print(f"{'=' * 60}")

        candidates = collect_candidates()
        print(f"Total candidates in this round: {len(candidates)}")

        new_candidates = [c for c in candidates if c not in sent_set]
        print(f"New candidates (not yet sent): {len(new_candidates)}")

        if not new_candidates:
            print("No new candidates to process. Stopping.")
            break

        results, sent_in_round = process_candidates(new_candidates, sent_set, cfg)
        total_sent.extend(sent_in_round)

        if sent_in_round:
            save_sent(sent_set)
            print(f"\nRound {round_num} sent to: {', '.join(sent_in_round)}")

        print("Waiting 5s for list refresh...")
        time.sleep(5)

        if not switch_job(cfg["target_job"], cfg["filter_status"], cfg.get("unread_only", True)):
            print("Failed to refresh job filter, stopping.")
            break
        time.sleep(2)

    print(f"\n{'=' * 60}")
    print("FINAL SUMMARY")
    print(f"{'=' * 60}")
    print(f"Total rounds           : {round_num}")
    print(f"Messages sent (session): {len(total_sent)}")
    print(f"Sent this session      : {', '.join(total_sent)}")
    print(f"Cumulative sent log    : {SENT_LOG_FILE}")
    print(f"Config file            : {CONFIG_FILE}")


if __name__ == "__main__":
    main()
