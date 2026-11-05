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

## So cấu hình giữa các agent

```text
wj check
```

Với Claude, Codex, Antigravity, Cursor và OpenCode, lệnh cho biết đã cài chưa, đang kết
nối kiểu gì, có MCP server và skill nào, và agent nào đang thiếu so với agent khác.

Lệnh chỉ đọc, không cần mạng, và chỉ in tên, không in key.
