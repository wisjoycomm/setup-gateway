package main

import (
	"fmt"
	"io"
	"strings"
)

const helpMain = `setup-gateway - cài và kết nối công cụ lập trình AI cho máy mới

CÁCH DÙNG NHANH
  setup-gateway                 mở menu, chọn bằng số (dễ nhất)
  setup-gateway help CHỦ_ĐỀ     xem hướng dẫn chi tiết một chủ đề

CHỦ ĐỀ HƯỚNG DẪN
  help claude    kết nối Claude Code (gateway, token OAuth, API key, đăng nhập web)
  help codex     kết nối Codex CLI / OpenCode
  help proxy     đo token đã dùng bằng mitmproxy
  help apps      cài / cập nhật ứng dụng
  help tools     bộ công cụ dòng lệnh, quản lý cửa sổ
  help words     giải thích các từ hay gặp (gateway, token, profile...)

LỆNH HAY DÙNG
  setup-gateway apps list                   xem app nào đã cài
  setup-gateway apps install claude,codex   cài app còn thiếu
  setup-gateway configure claude            kết nối Claude Code
  setup-gateway configure codex             kết nối Codex
  setup-gateway -use-profile adnx -run      đổi sang kết nối "adnx" và mở Claude
  setup-gateway proxy run claude            mở Claude qua mitmproxy (tự cài, tự bật)
  setup-gateway proxy usage                 xem token đã dùng trong 24 giờ
  setup-gateway update                      cập nhật chính tool này
  setup-gateway version                     xem phiên bản

Thêm -dry-run vào hầu hết lệnh để xem trước, không thay đổi gì.`

const helpClaude = `KẾT NỐI CLAUDE CODE

  setup-gateway configure claude    (hoặc: setup-gateway auth) - hỏi từng bước

Bạn đang có gì?                                    Chọn
  Domain công ty (vd ai.adnx.vn) + key của domain   1. Gateway
  Token bắt đầu bằng sk-ant-oat...                  2. Token OAuth
  API key từ console.anthropic.com (sk-ant-api...)  3. API key
  Tài khoản Claude Pro/Max, đăng nhập trên web      4. Đăng nhập web

Lệnh có sẵn tham số (không phải trả lời câu hỏi):
  Gateway:   setup-gateway auth -auth gateway -only claude -base-url ai.adnx.vn -profile adnx
  OAuth:     setup-gateway auth -auth oauth -profile work
  API key:   setup-gateway auth -auth api-key -profile api
  Web:       setup-gateway auth -auth login

Lưu & đổi kết nối (profile):
  -profile TÊN          lưu kết nối vừa nhập với tên này
  -list-profiles        xem các kết nối đã lưu (không hiện key)
  -use-profile TÊN      chuyển sang kết nối đã lưu
  -run                  mở Claude ngay sau khi xong

Tham số khác:
  -install-claude       tự cài Claude Code nếu máy chưa có
  -auth-header x-api-key  khi gateway đòi header x-api-key thay vì Bearer
  -skip-check -models ID1,ID2   gateway không có /v1/models: tự nhập tên model
  -key-stdin            đọc key từ stdin (cho script), không hiện ra màn hình
  -no-login             (đăng nhập web) chỉ dọn cấu hình, chưa mở trình duyệt
  -dry-run              xem trước, không ghi file, không gọi mạng

Model qua gateway: tool lấy danh sách model của gateway, gán Opus/Sonnet/Haiku/Fable
bản mới nhất, và bật CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY để /model trong
Claude hiện đủ model gateway đang có. Mỗi lần -use-profile, danh sách model được
lấy lại từ gateway và lưu vào profile.

Sau khi đổi: mở phiên Claude MỚI và gõ /status để kiểm tra.
Tool giữ nguyên hooks, plugin, statusLine; file cũ được sao lưu .bak-<thời gian>.

Lỗi hay gặp: dán token sk-ant-oat vào mục Gateway -> báo 401. Token OAuth phải
chọn mục 2. Cookie sessionKey (sk-ant-sid...) của trình duyệt KHÔNG dùng được.`

const helpCodex = `KẾT NỐI CODEX CLI / OPENCODE

  setup-gateway configure codex      hỏi từng bước
  setup-gateway configure opencode

Codex có 3 cách:
  1. Đăng nhập ChatGPT     setup-gateway configure codex -auth login
                           (máy không mở được trình duyệt: thêm -device-auth)
  2. API key OpenAI        setup-gateway configure codex -auth api-key
  3. Gateway công ty       setup-gateway configure codex -auth gateway -base-url ai.adnx.vn
     Dùng lại gateway đã lưu cho Claude (khỏi nhập key lần nữa):
                           setup-gateway configure codex -from-profile adnx

  Gateway cho Codex phải hỗ trợ /v1/responses. Tool lấy TẤT CẢ model của gateway
  đưa vào /model của Codex (GPT mới nhất đứng đầu, làm mặc định). Chạy lại lệnh
  bất cứ lúc nào để cập nhật khi gateway thêm model.
  Chỉ muốn một model: -models cx/gpt-6.1-sol
  Gateway không có /v1/models: thêm -skip-check -models TÊN_MODEL

  Cấu hình ghi vào ~/.codex/config.toml và ~/.codex/setup-gateway-models.json
  (giữ projects, MCP, hooks; có sao lưu).
  Token Claude (sk-ant-...) KHÔNG dùng cho Codex.

OpenCode:
  -auth login                          đăng nhập trong OpenCode
  -auth api-key -provider-id openai    key OpenAI trực tiếp
  -auth api-key -provider-id anthropic key Anthropic trực tiếp
  -auth gateway -base-url DOMAIN       gateway công ty (lấy đủ model của gateway)
  -from-profile adnx                   dùng lại gateway đã lưu cho Claude

Cursor, Antigravity, Orca tự đăng nhập trong app; configure chỉ in hướng dẫn.`

const helpApps = `CÀI / CẬP NHẬT ỨNG DỤNG

  setup-gateway apps list                     app nào đã cài, app nào thiếu (không cần mạng)
  setup-gateway apps install claude,codex     cài app còn thiếu (bỏ qua app đã có)
  setup-gateway apps install all -dry-run     xem sẽ cài gì
  setup-gateway apps update codex,orca        cập nhật (chỉ khi bạn gọi lệnh này)

Tên app: claude, codex, opencode, cursor-agent, cursor, antigravity (agy),
         antigravity-ide, orca, mitmproxy

Windows cần WinGet cho opencode/cursor/antigravity-ide/mitmproxy; macOS cần Homebrew.
Cài xong mà gõ lệnh không thấy: đóng terminal, mở cái mới.`

const helpTools = `BỘ CÔNG CỤ DÒNG LỆNH & QUẢN LÝ CỬA SỔ

  setup-gateway tools list                      xem có những nhóm nào
  setup-gateway tools install cli               cài (hỏi chọn nhóm)
  setup-gateway tools install cli -groups 1,2,4 chọn sẵn nhóm
  setup-gateway tools install wm                quản lý cửa sổ kiểu i3
  setup-gateway tools uninstall cli | wm        gỡ

Nhóm: 1 terminal & giao diện   2 tiện ích (rg, fd, bat, lazygit...)
      3 git, node, python, neovim   4 Claude Code, Codex, Antigravity CLI`

const helpWords = `GIẢI THÍCH TỪ NGỮ

  Gateway / custom domain  Máy chủ trung gian của công ty (vd ai.adnx.vn) cấp key riêng
                           và chuyển tiếp tới nhiều model (Claude, GPT, DeepSeek...).
  Token OAuth (sk-ant-oat) Token dài hạn của tài khoản Claude Pro/Max, thường do
                           admin cấp hoặc tạo bằng "claude setup-token".
  API key                  Key trả tiền theo lượng dùng, từ console.anthropic.com
                           hoặc platform.openai.com.
  Profile                  Một kết nối Claude đã lưu có tên (vd adnx, work) để đổi nhanh.
  Model ID                 Tên model gateway dùng, vd cc/claude-opus-5-5, cx/gpt-6.1-sol.
  Proxy / mitmproxy        Chương trình đứng giữa app và gateway để đếm token đã dùng.
  Dry run (-dry-run)       Chạy thử: chỉ in ra sẽ làm gì, không thay đổi máy.`

func printHelp(args []string, input io.Reader, out io.Writer) error {
	topic := ""
	if len(args) > 0 {
		topic = strings.ToLower(args[0])
	}
	switch topic {
	case "":
		fmt.Fprintln(out, helpMain)
	case "claude", "auth":
		fmt.Fprintln(out, helpClaude)
	case "codex", "opencode", "configure":
		fmt.Fprintln(out, helpCodex)
	case "proxy", "mitmproxy", "usage":
		fmt.Fprintln(out, proxyHelp)
	case "apps", "app":
		fmt.Fprintln(out, helpApps)
	case "tools", "tool":
		fmt.Fprintln(out, helpTools)
	case "words", "terms", "tu":
		fmt.Fprintln(out, helpWords)
	case "flags", "all":
		fmt.Fprintln(out, "Tất cả tham số kết nối Claude:")
		return runCLI([]string{"-help"}, input, out)
	default:
		return fmt.Errorf("không có chủ đề %q; gõ setup-gateway help để xem danh sách", topic)
	}
	return nil
}
