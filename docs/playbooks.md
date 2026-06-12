# BOSS-cli 筛选 Playbooks

## 新招呼初筛（含学校 tier 筛选）

**目标**：对指定岗位的"新招呼 + 未读"候选人，逐个查看在线简历，按 **关键词 + 毕业年份 + 学校 tier** 综合判断是否发送消息。

### 命令模板

```bash
./BOSS-cli scan-resumes "岗位名称" \
  --filter="新招呼" \
  --unread \
  --keyword="关键词" \
  --message="要发送的消息" \
  --ocr \
  --exclude-job-title \
  --min-grade=2027 \
  --school-tier=985 \
  --max=0
```

### 参数说明

| 参数 | 含义 | 示例 |
|---|---|---|
| `岗位名称` | 要筛选的岗位，必须和 BOSS 页面显示的一致 | `"Product Engineer Intern（AI Agent方向）"` |
| `--filter` | 沟通状态筛选 | `"新招呼"` |
| `--unread` | 只筛未读消息 | 布尔开关 |
| `--keyword` | 逗号分隔的关键词，命中任意一个即匹配 | `"agent,AI Agent,大模型"` |
| `--message` | 匹配后自动发送的消息 | `"求简历"` 或 `"求发简历和作品集"` |
| `--ocr` | 开启 OCR 识别在线简历 canvas 内容（macOS only） | 布尔开关 |
| `--exclude-job-title` | 过滤掉仅因岗位名产生的噪音匹配 | 布尔开关 |
| `--min-grade` | 最低毕业年份，从简历文本中提取 | `--min-grade=2027` |
| `--school-tier` | 学校 tier 预设：`c9` / `985` / `overseas` | `--school-tier=985` |
| `--schools` | 额外的自定义学校关键词（和 `--school-tier` 可叠加） | `--schools="清华,北大,港大"` |
| `--max` | 最大扫描人数，`0` 表示不限制 | `--max=50` |

### 学校 Tier 说明

| Tier | 覆盖范围 |
|---|---|
| `c9` | C9 联盟（清华、北大、复旦、上交、南大、浙大、中科大、哈工大、西交大） |
| `985` | 全部 39 所 985 高校（含 C9） |
| `overseas` | 海外及港澳台高校，通过英文 institution 关键词 + 常见海外校名识别 |

### 执行逻辑

1. 切换到指定岗位 + "新招呼" + "未读"。
2. 收集该状态下的所有候选人。
3. 对每个候选人：
   - 点开在线简历；
   - OCR 识别简历正文；
   - 用 `--keyword` 搜索关键词；
   - 提取毕业年份（支持"2027届"/"27届"/"2027年毕业"/"27年应届生"等）；
   - 提取学校（匹配 `--school-tier` 和 `--schools` 合并后的关键词库）；
   - 若 **关键词命中** 且 **毕业年份 >= min-grade** 且 **学校匹配**，则发送 `--message`；
   - 任一条件不满足，继续下一个人。
4. 输出每个人的匹配结果、毕业年份、学校和发送状态。

### 常用组合示例

**示例 1：AI Agent 产品岗，985，27 届及以后，关键词 agent**

```bash
./BOSS-cli scan-resumes "Product Engineer Intern（AI Agent方向）" \
  --filter="新招呼" --unread \
  --keyword="agent" --message="求简历" \
  --ocr --exclude-job-title \
  --min-grade=2027 --school-tier=985 --max=0
```

**示例 2：动态视觉设计岗，C9/985/海外，27 届及以后，关键词 AE**

```bash
./BOSS-cli scan-resumes "Kimi 动态视觉设计实习生" \
  --filter="新招呼" --unread \
  --keyword="AE" --message="求发简历和作品集" \
  --ocr --exclude-job-title \
  --min-grade=2027 --school-tier=overseas --schools="清华,北大,复旦,上交,浙大,南大,中科大" --max=0
```

### 注意事项

- OCR 每个候选人约 5–15 秒，大量候选人时请合理设置 `--max` 或放后台执行。
- 岗位名称必须准确，建议先用 `./BOSS-cli list-jobs` 确认。
- `--exclude-job-title` 会过滤包含完整岗位名的行，避免岗位名本身带关键词造成的误匹配。
- 学校识别依赖 OCR 文本，如果简历里学校名称写得不规范（如只写"北大"而非"北京大学"），tier 关键词库已包含常见简称，一般能识别。
- 发送消息后，候选人的未读状态会变化，重复扫描同一状态时可能找不到人。
