# BOSS-cli

BOSS Zhipin (BOSS直聘) automation CLI built on top of [kimi-webbridge](https://github.com/moonshot-ai/kimi-webbridge). It controls your real Chrome browser to automate recruiter workflows: list jobs, switch filters, browse candidates, send messages, and download resumes.

> **Warning**: This tool automates interactions with the BOSS Zhipin web interface. Use it responsibly and in compliance with BOSS Zhipin's Terms of Service. The authors are not responsible for any account restrictions.

---

## Features

- **Session reuse** — works with your already-logged-in Chrome, no password handling
- **JSON output** — every command prints structured JSON, easy to pipe into `jq` or scripts
- **3D job switching** — switch job + communication status filter + unread in one command, with verification
- **Candidate screening** — list and filter candidates in the chat page
- **Message automation** — send messages with a reliable DOM event sequence
- **Resume download** — click and save candidate resumes to a target directory

---

## Prerequisites

1. **macOS** (primary target; Linux/Windows may work with minor tweaks)
2. **Chrome** with the [kimi-webbridge](https://github.com/moonshot-ai/kimi-webbridge) extension installed
3. **kimi-webbridge daemon** running on `127.0.0.1:10086`
4. **Go 1.22+** to compile the CLI
5. **Python 3** (optional) for the example screening scripts

### Start kimi-webbridge

```bash
kimi-webbridge start

# Verify
kimi-webbridge status
```

### Log in to BOSS Zhipin

Open Chrome, navigate to https://www.zhipin.com/web/chat/index and log in manually. The CLI reuses your existing browser session.

---

## Installation

### From source

```bash
git clone https://github.com/Rhypoo-Ma/BOSS-cli.git
cd BOSS-cli
make build
```

### Using go install

```bash
go install github.com/Rhypoo-Ma/BOSS-cli@latest
```

---

## CLI Commands

All commands output JSON to stdout. Use `--help` on any command for details.

```bash
BOSS-cli --help
```

### Check login status

```bash
./BOSS-cli login-status
```

### List jobs

```bash
./BOSS-cli list-jobs
```

### Switch job, filter, and unread state

`switch-job` is a **three-dimensional state switch**: job (岗位) × communication status (沟通状态) × message status (消息状态: 全部/未读). It switches each dimension and verifies the result before returning.

```bash
# Job + communication filter + unread
./BOSS-cli switch-job "商业分析实习生" --filter="新招呼" --unread

# Job + communication filter only
./BOSS-cli switch-job "海外社媒增长运营实习生" --filter="沟通中"

# Job only
./BOSS-cli switch-job "海外社媒增长运营实习生"
```

| Dimension | Options |
|---|---|
| Job (岗位) | Any job name from `list-jobs` |
| Communication status (沟通状态) | `全部` / `新招呼` / `沟通中` / `已约面` / `已获取简历` / `已交换电话` / `已交换微信` / `收藏` / `更多` |
| Message status (消息状态) | `全部` (default) / `未读` (`--unread`) |

Each dimension is retried independently if the target state is not reached. Use `--debug` to print a page snapshot on failure:

```bash
./BOSS-cli switch-job "商业分析实习生" --filter="新招呼" --unread --debug
```

### List candidates

```bash
./BOSS-cli list-candidates
./BOSS-cli list-candidates --status="沟通中"
```

### Send a message

```bash
./BOSS-cli send-message "候选人姓名" "你好，感谢投递！"
```

### Download resume

```bash
./BOSS-cli download-resume "候选人姓名" --dir="./resumes"
```

### View online resume

Open a candidate's online resume dialog, extract the visible preview info, and close it automatically:

```bash
./BOSS-cli view-resume "候选人姓名"
```

Keep the resume dialog open so you can scroll through it in the browser:

```bash
./BOSS-cli view-resume "候选人姓名" --keep-open
./BOSS-cli scroll-resume 800
./BOSS-cli close-resume
```

Search the resume for a keyword (case-insensitive) and get structured matches:

```bash
./BOSS-cli view-resume "候选人姓名" --keyword="AI"
```

BOSS renders the detailed online resume body via WebAssembly/canvas, so plain text extraction only covers the summary panel. To search the actual rendered resume (including work experience descriptions), use OCR on macOS:

```bash
./BOSS-cli view-resume "候选人姓名" --keyword="AI" --ocr
```

You can also pass comma-separated synonyms so that any of them counts as a match:

```bash
./BOSS-cli view-resume "候选人姓名" --keyword="达人,红人,KOL,创作者" --ocr
```

To ignore matches that only come from the job title (e.g. "AI达人营销"):

```bash
./BOSS-cli view-resume "候选人姓名" --keyword="达人,红人,KOL,创作者" --ocr --exclude-job-title
```

The extracted preview includes name, age, work years, education, work experience, and education history.

### Show version

```bash
./BOSS-cli version
```

---

## Python Examples

The `examples/` directory contains reference scripts that show how to build screening automation on top of the daemon API directly.

```bash
# First set the right job/filter via CLI
./BOSS-cli switch-job "岗位名" --filter="新招呼" --unread

# Then run a screening script
python3 examples/bulk_screen_and_send.py
```

## Advanced Workflow Scripts

The `scripts/` directory contains ready-to-use workflow scripts built on top of the CLI and daemon.

### `screen_loop_generic.py`

A configurable auto-screener that:
1. Switches to the configured job + filter + unread state
2. Scrolls through the candidate virtual list
3. Matches candidates by school, company, or keywords
4. Sends the configured message to matches
5. Records already-sent candidates to avoid duplicates

Copy the example config and customize it:

```bash
cp scripts/screen_loop_config.example.json scripts/screen_loop_config.json
# Edit scripts/screen_loop_config.json with your job, filters, and message
python3 scripts/screen_loop_generic.py
```

---

## Project Structure

```
BOSS-cli/
├── cmd/                  # Cobra CLI commands
│   ├── root.go
│   └── version.go
├── boss/                 # BOSS Zhipin page automation logic
│   ├── login.go
│   ├── jobs.go
│   ├── filters.go
│   ├── candidates.go
│   ├── message.go
│   ├── resume.go
│   └── online_resume.go
├── browser/              # HTTP client for kimi-webbridge daemon
│   └── client.go
├── output/               # JSON output helpers
│   └── output.go
├── examples/             # Python screening examples
├── scripts/              # Ready-to-use workflow scripts
│   ├── screen_loop_generic.py
│   └── screen_loop_config.example.json
├── main.go
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Development

```bash
make build    # build the binary
make test     # run tests
make vet      # run go vet
make clean    # remove build artifacts
```

---

## Troubleshooting

| Issue | Solution |
|---|---|
| `daemon unreachable` | Start `kimi-webbridge` and check `kimi-webbridge status` |
| `not_logged_in` | Log in manually at https://www.zhipin.com/web/chat/index |
| Candidate disappears after scroll | BOSS uses a virtual list; process only currently visible items |
| Message not delivered | Check chat history for `[送达]`. The submit button may need another activation event |
| Extension disconnected | Refresh the BOSS page in Chrome and wait a few seconds |

---

## License

[MIT](./LICENSE)

---

## Author

[Rhypoo Ma](https://github.com/Rhypoo-Ma)
