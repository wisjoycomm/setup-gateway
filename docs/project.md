# Project

## Setup ban đầu

```text
cd <project>
wj init                    tự nhận job, hoặc hỏi
wj init -type unity,web    chọn job
wj init -dry-run           xem trước
```

Job có sẵn: `unity`, `web`, `go`, `python`.

Tool thêm rule chung, rule theo job, `PLAN.md`, quyền cho `.claude/settings.json`,
`.gitignore` và skill Orca. Chạy lại nhiều lần được: file đã có không bị ghi đè, còn rule
trong `AGENTS.md` nằm giữa hai dấu `<!-- wj:begin -->` nên phần tự viết được giữ nguyên.

Xong thì commit các file này, vì worker Orca chỉ thấy file đã commit.

## PLAN.md: một bảng duy nhất

`PLAN.md` chỉ có một bảng Markdown, mỗi task một dòng, không có gì khác (không tiêu đề, không
ghi chú, không danh sách). Nhờ vậy script lấy một task chỉ cần đọc đúng một dòng.

```text
| ID | Task | Pri | SP | Status | Agent | Depends | Scope | Verify | Notes | Session |
|---|---|---|---|---|---|---|---|---|---|---|
| T-01 | Thêm kiểm tra JWT | P1 | 3 | DOING | claude | - | `src/auth.py` | `pytest -q` | - | wt-auth |
| T-02 | Sửa timeout pool DB | P0 | 2 | BLOCKED | codex | T-01 | `src/db/` | - | timeout sau 3 lần retry | wt-db |
```

| Cột | Giá trị |
|---|---|
| Pri | `P0` gấp, `P1` cao, `P2` thường, `P3` để sau (thay cho Backlog) |
| SP | Story point kiểu agile: `1` `2` `3` `5` `8` `13`, hoặc `?` chưa ước lượng. Từ 8 nên tách nhỏ |
| Status | `TODO` `DOING` `DONE` (chưa merge) `BLOCKED` `PARTIAL` |
| Agent | `claude` `codex` `agy` |
| Depends | ID cách nhau dấu phẩy, hoặc `-` |
| Scope | Đường dẫn trong backtick, hoặc `-` |
| Verify | Một lệnh trong backtick, hoặc `-` để tự đoán |
| Notes | Tối đa 100 ký tự |
| Session | Phiên/worktree đang làm task, hoặc `-` |

Không ô nào chứa ký tự `|` hay xuống dòng. Chỉ orchestrator sửa `PLAN.md`, và sửa đúng một dòng
mỗi lần; task mới thêm ở cuối, ID tăng dần, không đánh lại số. Task đã merge chuyển sang
`PLAN-ARCHIVE.md`.

```text
wj plan            task sẵn sàng chạy (ưu tiên cao trước), đang chạy, bị chặn
wj plan check      kiểm tra format, in lỗi kèm số dòng
wj plan convert    đổi PLAN.md kiểu cũ (checkbox + Backlog) sang bảng, cập nhật skill Orca
                   của project; bản cũ được sao lưu .bak-<giờ>
wj plan archive T-01 T-02                 chuyển task đã merge sang PLAN-ARCHIVE.md
wj plan archive T-01 -commit 9fb158c      ghi rõ commit merge
wj plan archive T-01=9fb158c T-02=3f38ef1 mỗi task một commit
```

`PLAN-ARCHIVE.md` cũng là bảng: đủ các cột của `PLAN.md`, thêm `Commit` (commit merge) và
`Archived` (ngày). Không ghi `-commit` thì tool lấy commit mới nhất trong `git log` có nhắc ID
task; không tìm thấy thì ô Commit là `-`. Chữ cũ trong `PLAN-ARCHIVE.md` được giữ nguyên, bảng nối
vào cuối. Chỉ task `DONE` hoặc `PARTIAL` mới archive được.

Khi đổi kiểu cũ, tiêu đề dài bị cắt còn 120 ký tự (bản đầy đủ ở file .bak), Pri mặc định `P1`
cho task, `P2` cho dòng lẻ trong Queue, `P3` cho Backlog, và SP là `?`: chỉnh lại sau.

## Tự kiểm tra máy: wj doctor

```text
wj doctor              kiểm tra, rồi hỏi sửa từng mục (Enter = sửa)
wj doctor -yes         sửa hết không hỏi
wj doctor -dry-run     chỉ báo
wj doctor -ai          nhờ AI qua gateway của bạn gợi ý thêm
```

Lệnh tìm:

- Phiên Claude bị tắt tool search giữa chừng (đọc cache tăng vọt, xem [Model](model.md)).
  Sửa: cho project đi qua proxy wj, bật proxy chạy nền.
- Proxy đang chạy addon cũ. Sửa: ghi addon mới, khởi động lại proxy nền.
- Project bật đo token mà proxy không chạy (Claude ở đó không kết nối được). Sửa: bật proxy nền.
- MCP cài cho mọi project nhưng chỉ vài project dùng trong 30 ngày. Sửa: chuyển về đúng các
  project đó bằng chính lệnh `claude mcp`, không gọi được thì bỏ khỏi phạm vi chung.
- `PLAN.md` kiểu cũ hoặc sai format.

`-ai` chỉ gửi các dòng báo cáo (đường dẫn, số đếm) lên gateway trong `~/.claude/settings.json`,
không gửi key hay code. Không có gateway hoặc mất mạng thì vẫn chạy với các luật sửa cứng.

Proxy nền: `wj proxy bg` bật, `wj proxy stop` tắt, `wj proxy restart` nạp addon mới.

## So cấu hình giữa các agent

```text
wj check
```

Với Claude, Codex, Antigravity, Cursor và OpenCode, lệnh cho biết đã cài chưa, đang kết
nối kiểu gì, có MCP server và skill nào, và agent nào đang thiếu so với agent khác.

Lệnh chỉ đọc, không cần mạng, và chỉ in tên, không in key.
