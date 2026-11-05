# Model từ gateway

## Tự động lấy danh sách

`wj configure`, `wj auth` và `-use-profile` luôn lấy danh sách mới từ gateway. Mỗi gateway
đặt tên một kiểu, nên tool giữ nguyên từng tên trong danh sách, không sửa và không đòi tên
có `claude` hay `gpt`. Tên trong các ví dụ ở đây chỉ để minh họa.

| App | Model Claude | Model khác |
|---|---|---|
| Claude Code | Tự cập nhật mỗi lần mở app | Cập nhật khi chạy các lệnh ở đây |
| Codex | Cập nhật khi chạy các lệnh ở đây | Cập nhật khi chạy các lệnh ở đây |

Gateway vừa thêm model thì chạy:

```text
wj models
```

Lệnh dùng key đã lưu trong từng app, giữ nguyên model mặc định, và không hỏi lại key.

## Dùng chéo model

Claude Code chỉ gửi `/v1/messages`, Codex chỉ gửi `/v1/responses`. Cả hai app đều chọn
được model khác hãng, nhưng gateway phải tự chuyển đổi đường gửi đó.

Hãng không hỗ trợ chính thức việc này, nên có thể lỗi ở tool, suy luận hoặc nén ngữ cảnh.
Thử thật trước khi dùng:

```text
wj models test                 thử mọi model, tốn vài token, có hỏi trước
wj models test -models a,b     chỉ thử vài model
```

```text
MODEL               /v1/messages (Claude Code)  /v1/responses (Codex)
cc/claude-opus-5-5  OK                          HTTP 404
cx/gpt-6.1-sol      HTTP 429                    OK
```

`OK` ở cột nào thì app đó chọn model này được. Lệnh chỉ thử một câu ngắn có tool, không
đảm bảo mọi tính năng của app.
