# Token

Đọc thẳng file phiên của Claude Code và Codex trên máy này. Không cần proxy, không gọi
mạng, tính cả phiên mở từ Orca. Các worktree của một repo gộp về một project.

## Lệnh

```text
wj top                    bảng trực tiếp, project vừa chạy lên đầu (Ctrl+C để thoát)
wj top -once              in một lần
wj overlay                cửa sổ nhỏ luôn nổi trên màn hình
wj overlay stop           đóng overlay
wj usage                  bảng chi tiết theo project + app
wj usage -by app          theo app + model; còn có -by project, -by model
wj usage -project game -app codex
```

`-since 168h` đổi khoảng thời gian (mặc định 24 giờ). `wj usage -all` lấy toàn bộ.

## Tổng cả máy

`wj top` có dòng tổng mọi project:

```text
Cả máy: 24 giờ 7.7M · 7 ngày 173.6M · 30 ngày 812.1M · toàn bộ 986.3M
```

Overlay hiện tổng cả máy trong khoảng đang xem, thêm 7 ngày và 30 ngày.

## Overlay

Chỉ cần mở một lần, không phải setup hay thêm tay từng project. Project mới và session
đang chạy dở tự hiện sau tối đa 5 giây, không cần khởi động lại session.

Hiện tối đa 20 project:

- **Claude Code**: session đang mở, kể cả khi chưa dùng token.
- **Codex**: có dùng token trong 10 phút gần nhất. Codex không ghi lại session đang mở, nên
  session Codex nghỉ lâu hơn sẽ tạm ẩn cho đến lượt dùng tiếp theo.

Chấm xanh là project vừa dùng token trong 2 phút. Số token của mỗi dòng tính trong khoảng
đang xem (mặc định 24 giờ).

Kéo để đổi chỗ, chuột phải để đóng. Vị trí được lưu cho lần mở sau. Hiện chỉ có trên
Windows; macOS và Linux dùng `wj top` trong một cửa sổ terminal nhỏ.

## Đọc số thế nào

- **VÀO**: token prompt mới.
- **RA**: token trả lời.
- **ĐỌC CACHE**: thường rẻ hơn nhiều so với VÀO, và thường chiếm phần lớn TỔNG.
- **TỔNG**: cộng cả bốn loại, nên không phải số tiền. Giá thật do gateway quyết định.

## So với /usage của Claude Code

| | `/usage` | `wj` |
|---|---|---|
| Cho biết | Còn bao nhiêu % hạn mức gói | Đã dùng bao nhiêu token |
| Theo project | Không | Có |
| Codex | Không | Có |
| Biết sắp hết hạn mức | Có | Không |
| Tính cả máy khác | Có, theo tài khoản | Không, chỉ máy này |

## Giới hạn

- Chỉ tính được khi file phiên còn trên máy.
- Số liệu của từng máy riêng, không cộng giữa các máy.
- App khác (OpenCode...) đo bằng `wj proxy run` và `wj proxy usage`. Xem `wj help proxy`.
