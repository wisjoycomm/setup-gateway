# Kết nối

Key nhập riêng trên từng máy. Tool không in key ra màn hình.

## Claude Code

```text
wj configure claude                                  hỏi từng bước
wj auth -auth gateway -only claude -base-url ai.adnx.vn -profile adnx
wj auth -auth oauth -profile work                    token sk-ant-oat...
wj auth -auth api-key -profile api                   key từ console.anthropic.com
wj auth -auth login                                  đăng nhập trên web
```

Đổi kết nối đã lưu:

```text
wj -list-profiles
wj -use-profile adnx -run
```

Sau khi đổi, mở phiên Claude mới và gõ `/status` để kiểm tra.

## Codex và OpenCode

```text
wj configure codex                                   hỏi từng bước
wj configure codex -auth gateway -base-url ai.adnx.vn
wj configure codex -from-profile adnx                dùng lại gateway đã lưu cho Claude
wj configure opencode
```

Gateway cho Codex phải hỗ trợ `/v1/responses`.

Cursor, Antigravity và Orca tự đăng nhập trong app; `wj configure` chỉ in hướng dẫn.

File cấu hình cũ được sao lưu thành `.bak-<thời gian>` trước khi ghi đè.
