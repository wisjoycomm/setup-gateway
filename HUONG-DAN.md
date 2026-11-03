# Huong dan cai may moi - Wisjoy setup-gateway

Mot tool CLI gom bo `Bo_cong_cu` va phan cau hinh Claude/gateway.
Ban tai mot file executable, khong can cai Go hay Node de mo tool.

## 1. Cach de nhat tren Windows

Mo **CMD**, dan mot lenh nay:

```bat
powershell -NoProfile -ExecutionPolicy Bypass -Command "& ([scriptblock]::Create((irm 'https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.ps1'))) -RunSetup"
```

Tool tu tai ban release, kiem tra SHA-256, cai vao PATH va mo menu ngay.
Khong can tu tai ZIP hay giai nen. Lan sau mo CMD moi va go `setup-gateway`.
Neu CMD/Windows Terminal dang mo tu truoc, dong han va mo lai de nhan PATH moi.

Neu muon chuyen file sang may khac, van co cach dung ZIP:

1. Vao muc **Releases** cua repo va tai `setup-gateway-windows-amd64.zip`.
2. Giai nen ra mot thu muc.
3. Chay `Run-Setup.cmd`.
4. Chon `1` de cai may moi, hoac chon tung muc rieng.

Neu chua phat hanh ban moi: dung ZIP trong thu muc `dist` cua nguoi build.
Neu chi co source Git, xem muc Build o cuoi file nay.

Menu:

```text
1. Cai dat may moi (cong cu + Claude)
2. Ket noi Claude: OAuth / API key / gateway / login
3. Cai bo cong cu dong lenh
4. Cai quan ly cua so
5. Go bo cong cu dong lenh
6. Go quan ly cua so
7. Xem / chon profile Claude da luu
0. Thoat
```

Che do may moi hoi tung phan. Khong can cai het. Phan quan ly cua so la tuy chon.
Khi cai package, Windows co the hien UAC; Homebrew/Linux co the hoi mat khau sudo.

## 2. Cai lenh vao PATH

Sau khi ban moi da duoc phat hanh, mo PowerShell:

```powershell
& ([scriptblock]::Create((irm 'https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.ps1'))) -RunSetup
```

Installer tu chon ban Windows x64/ARM64 va kiem tra SHA-256.
File duoc cai vao `%USERPROFILE%\.local\bin\setup-gateway.exe`.
Lenh tren tai ban **release da phat hanh**, khong tu build source tren nhanh Git.

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.sh | sh
~/.local/bin/setup-gateway
```

## 3. Cai bo cong cu cua ban

```powershell
setup-gateway tools list
setup-gateway tools install cli
```

| Nhom | Cong cu |
| --- | --- |
| 1 | Terminal, Starship, Nerd Font, fzf, zoxide; Windows co PowerShell 7, Windows Terminal, Clink |
| 2 | rg, fd, bat, eza, jq, lazygit, delta, btop, gh |
| 3 | Git, fnm/Node, uv/Python, Neovim, Antigravity IDE; Docker hoi rieng |
| 4 | Claude Code, Codex CLI, Antigravity CLI |

Chon san nhom:

```powershell
setup-gateway tools install cli -groups 1,2,4
```

Tool bo qua ung dung da co, them khoi cau hinh shell co danh dau, alias, prompt,
font va mot so mac dinh Git. Mo terminal moi sau khi cai de nhan PATH/profile.

Quan ly cua so:

```powershell
setup-gateway tools install wm
```

- Windows: Komorebi + whkd, phim Alt kieu i3, workspace theo man hinh, thanh bar.
- macOS: AeroSpace va phim tat tuong ung.
- Linux: cau hinh i3/Hyprland **da cai san**; module khong cai desktop session.

Phan nay thay doi shortcut, startup, taskbar/Dock va cach xep cua so.
Neu dang dung GlazeWM, tool hoi rieng truoc khi go de chuyen sang Komorebi.

## 4. Ket noi Claude - chon dung loai key

```powershell
setup-gateway auth
```

| Ban dang co | Chon |
| --- | --- |
| Token `sk-ant-oat...` do sep/admin cap | OAuth / session token |
| API key cua Anthropic Console | API key |
| Domain vi du `https://ai.adnx.vn` + key cua domain do | Gateway / custom domain |
| Co the dang nhap tai khoan Claude tren trinh duyet | Login |

Token duoc nhap an, khong in ra terminal. Neu may chua co Claude Code, wizard
hoi co cai bang installer chinh thuc hay khong.

### OAuth / session token

```powershell
setup-gateway auth -auth oauth -profile work -install-claude
```

Dan nguyen token `sk-ant-oat...` vao prompt.
Khong can dang nhap hay chay `claude setup-token` neu da duoc cap token hop le.
Tool se ghi `CLAUDE_CODE_OAUTH_TOKEN`.

`-auth session-key` cung co nghia nhu `-auth oauth` trong tool nay.
Cookie web `sessionKey` / `sk-ant-sid...` KHONG duoc ho tro.
Tool khong gui request de thu OAuth; Claude se kiem tra khi ban gui prompt dau tien.

### API key truc tiep

```powershell
setup-gateway auth -auth api-key -profile api -install-claude
```

Nhap key Anthropic Console. Day la API usage/billing cua key do, khong phai token
goi Pro/Max. Tool ghi `ANTHROPIC_API_KEY` va dung endpoint Anthropic chinh thuc.

### Custom domain / gateway ADNX

```powershell
setup-gateway auth -auth gateway -only claude -base-url https://ai.adnx.vn -profile adnx
```

Nhap key **do ADNX cap**. Khong dan token `sk-ant-oat` vao day.
Tool lay danh sach model, gan alias opus/sonnet/haiku/fable phu hop.

Neu gateway yeu cau header `x-api-key`:

```powershell
setup-gateway auth -auth gateway -only claude -base-url https://example.com -auth-header x-api-key
```

Neu gateway khong co API danh sach model:

```powershell
setup-gateway auth -auth gateway -only claude -base-url https://example.com -skip-check -models cc/claude-opus,cc/claude-sonnet,cc/claude-haiku
```

Thay cac model vi du bang ID that cua gateway. Neu model khong co quyen dung,
request van co the loi du da lay duoc danh sach model.

### Login binh thuong

```powershell
setup-gateway auth -auth login -install-claude
```

Tool don cau hinh key/gateway cu va mo `claude auth login --claudeai`.
Neu chi muon chuan bi cau hinh, chua dang nhap:

```powershell
setup-gateway auth -auth login -no-login
```

## 5. Doi giua ADNX va OAuth

Sau khi luu hai profile `adnx` va `work` nhu vi du tren:

```powershell
setup-gateway -list-profiles
setup-gateway -use-profile adnx -run
```

Lan sau doi sang OAuth:

```powershell
setup-gateway -use-profile work -run
```

Profile duoc luu trong `%USERPROFILE%\.setup-gateway\profiles`.
Khi doi, tool cap nhat settings Claude cua user, xoa key/base URL va model mapping
cu (vi du `cc/...`), giu hooks Orca, plugin, statusLine va cac tuy chon khac.
Tool sao luu settings truoc khi ghi.

Day la **chon mot ket noi dang dung**, khong phai tu dong chuyen du phong khi het
quota va khong tach rieng hai tai khoan chay dong thoi. Hay mo phien Claude moi
sau khi doi. Trong Claude go `/status` de xem nguon xac thuc va model.

`-run` mo Claude voi cac bien auth/model cu trong terminal da duoc loai khoi
process con. No khong xoa bien moi truong User/Machine va khong sua project hay
managed settings. Neu cong ty quan ly endpoint, chinh sach do van co hieu luc.

## 6. Loi 401 "API key required for remote API access"

Loi thuong gap: dat token `oat` vao `ANTHROPIC_AUTH_TOKEN` nhung van de
`ANTHROPIC_BASE_URL=https://ai.adnx.vn`. Request se toi gateway voi sai loai key.

De dung OAuth truc tiep:

```powershell
setup-gateway auth -auth oauth -profile work -run
```

Nhap lai token OAuth khi duoc hoi. Tool tu don URL ADNX va model `cc/...` cu.

Neu `/status` van hien endpoint cu, kiem tra settings cua project, bien moi truong
va managed settings; chung khong nam trong file user ma tool vua cap nhat.

## 7. Xem truoc va go cai dat

```powershell
setup-gateway tools install cli -groups 1,4 -dry-run
setup-gateway auth -auth oauth -dry-run
setup-gateway tools uninstall cli
setup-gateway tools uninstall wm
```

Dry-run khong ghi file, goi mang, cai/go package hoac mo login.
Lenh go co prompt xac nhan; voi CLI, co the chi xoa khoi shell ma giu ung dung.
Neu chon go package, ung dung da co tu truoc cung co the bi go.

File auth va profile chua key tren may: khong commit, gui kem ZIP hoac chia se
file backup cho nguoi khac. ZIP do `build.ps1` tao chi co binary + launcher + docs,
khong gom token/cau hinh ca nhan.

## 8. Tu build va phat hanh

Can Go theo `go.mod`. Trong thu muc repo:

```powershell
go test ./...
go vet ./...
.\build.ps1
```

Lay `dist\setup-gateway-windows-amd64.zip` gui sang may moi.
Dung `.\build.ps1 -All` de build them Windows ARM64, macOS va Linux.

CI kiem tra Go va cu phap script; khong tu cai/go ung dung tren may test.
Workflow Release tao binary, ZIP va checksum khi push tag `v*` da duoc duyet.
Push code len nhanh Git chua tu thay the ban release ma installer dang tai.
