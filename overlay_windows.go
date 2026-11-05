//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The overlay is a plain Win32 window: no CGO, no WebView, nothing to install.

const (
	overlayClass   = "WjTokenOverlay"
	overlayRefresh = 5 * time.Second

	wmDestroy       = 0x0002
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x0014
	wmNcHitTest     = 0x0084
	wmNcRButtonUp   = 0x00A5
	wmExitSizeMove  = 0x0232
	wmSnapshot      = 0x8001
	htCaption       = 2
	wsPopup         = 0x80000000
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	wsExLayered     = 0x00080000
	wsExNoActivate  = 0x08000000
	lwaAlpha        = 2
	swShowNoActive  = 4
	swpNoActivate   = 0x0010
	swpNoZOrder     = 0x0004
	swpNoMove       = 0x0002
	tpmReturnCmd    = 0x0100
	menuClose       = 1
	logPixelsY      = 90
	bgColor         = 0x1D1816
	textColor       = 0xEBE8E6
	mutedColor      = 0x99908A
	activeColor     = 0x80DE4A
	detachedProcess = 0x00000008
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	getModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	setProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	registerClassEx     = user32.NewProc("RegisterClassExW")
	createWindowEx      = user32.NewProc("CreateWindowExW")
	defWindowProc       = user32.NewProc("DefWindowProcW")
	showWindow          = user32.NewProc("ShowWindow")
	setLayeredAttrs     = user32.NewProc("SetLayeredWindowAttributes")
	setWindowPos        = user32.NewProc("SetWindowPos")
	getWindowRect       = user32.NewProc("GetWindowRect")
	getClientRect       = user32.NewProc("GetClientRect")
	getMessage          = user32.NewProc("GetMessageW")
	translateMessage    = user32.NewProc("TranslateMessage")
	dispatchMessage     = user32.NewProc("DispatchMessageW")
	postMessage         = user32.NewProc("PostMessageW")
	postQuitMessage     = user32.NewProc("PostQuitMessage")
	findWindow          = user32.NewProc("FindWindowW")
	destroyWindow       = user32.NewProc("DestroyWindow")
	invalidateRect      = user32.NewProc("InvalidateRect")
	beginPaint          = user32.NewProc("BeginPaint")
	endPaint            = user32.NewProc("EndPaint")
	getDC               = user32.NewProc("GetDC")
	releaseDC           = user32.NewProc("ReleaseDC")
	fillRect            = user32.NewProc("FillRect")
	loadCursor          = user32.NewProc("LoadCursorW")
	getSystemMetrics    = user32.NewProc("GetSystemMetrics")
	monitorFromRect     = user32.NewProc("MonitorFromRect")
	createPopupMenu     = user32.NewProc("CreatePopupMenu")
	appendMenu          = user32.NewProc("AppendMenuW")
	trackPopupMenu      = user32.NewProc("TrackPopupMenu")
	destroyMenu         = user32.NewProc("DestroyMenu")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")

	createFont          = gdi32.NewProc("CreateFontW")
	selectObject        = gdi32.NewProc("SelectObject")
	deleteObject        = gdi32.NewProc("DeleteObject")
	deleteDC            = gdi32.NewProc("DeleteDC")
	createCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBmp = gdi32.NewProc("CreateCompatibleBitmap")
	createSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	setBkMode           = gdi32.NewProc("SetBkMode")
	setTextColor        = gdi32.NewProc("SetTextColor")
	textOut             = gdi32.NewProc("TextOutW")
	getTextExtent       = gdi32.NewProc("GetTextExtentPoint32W")
	getDeviceCaps       = gdi32.NewProc("GetDeviceCaps")
	bitBlt              = gdi32.NewProc("BitBlt")

	dwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

type winRect struct{ Left, Top, Right, Bottom int32 }
type winSize struct{ CX, CY int32 }
type winPoint struct{ X, Y int32 }

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
	Private uint32
}

type winClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type winPaint struct {
	HDC      uintptr
	Erase    int32
	Paint    winRect
	Restore  int32
	IncUpd   int32
	Reserved [32]byte
}

type overlayLine struct {
	Project, Apps, Ago, Tokens string
	Active                     bool
}

type overlayView struct {
	mu         sync.Mutex
	footer     string
	title      string
	total      string
	lines      []overlayLine
	hwnd       uintptr
	font, bold uintptr
	scale      float64
	statePath  string
}

var overlay overlayView

func runOverlay(args []string, out io.Writer, home string) error {
	flags := flag.NewFlagSet("overlay", flag.ContinueOnError)
	flags.SetOutput(out)
	window := flags.Duration("since", 24*time.Hour, "khoảng thời gian, ví dụ 1h, 24h, 168h (7 ngày)")
	foreground := flags.Bool("run", false, "chạy cửa sổ ngay trong tiến trình này")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	class, _ := windows.UTF16PtrFromString(overlayClass)
	existing, _, _ := findWindow.Call(uintptr(unsafe.Pointer(class)), 0)
	switch {
	case flags.NArg() == 1 && flags.Arg(0) == "stop":
		if existing == 0 {
			fmt.Fprintln(out, "Overlay chưa mở.")
			return nil
		}
		postMessage.Call(existing, wmClose, 0, 0)
		fmt.Fprintln(out, "Đã đóng overlay.")
		return nil
	case flags.NArg() != 0:
		return errors.New("cách dùng: wj overlay [-since 24h] | wj overlay stop")
	}
	if existing != 0 {
		fmt.Fprintln(out, "Overlay đang mở rồi. Đóng: wj overlay stop")
		return nil
	}
	if *foreground {
		return overlayLoop(*window, home)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "overlay", "-run", "-since", window.String())
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | windows.CREATE_NEW_PROCESS_GROUP}
	if err = cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	fmt.Fprintln(out, "Đã mở overlay token. Kéo để đổi chỗ, chuột phải để đóng (hoặc: wj overlay stop).")
	return nil
}

func overlayLoop(window time.Duration, home string) error {
	runtime.LockOSThread()
	overlay.statePath = filepath.Join(stateDir(home), "overlay.json")
	overlay.title = "Cả máy, " + shortWindow(window)
	setProcessDPIAware.Call()
	instance, _, _ := getModuleHandle.Call(0)
	class, _ := windows.UTF16PtrFromString(overlayClass)
	cursor, _, _ := loadCursor.Call(0, 32512)
	wc := winClassEx{WndProc: windows.NewCallback(overlayProc), Instance: instance, Cursor: cursor, ClassName: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if atom, _, err := registerClassEx.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return fmt.Errorf("không tạo được cửa sổ: %w", err)
	}
	hwnd, _, err := createWindowEx.Call(wsExTopmost|wsExToolWindow|wsExLayered|wsExNoActivate,
		uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(class)), wsPopup, 0, 0, 10, 10, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("không tạo được cửa sổ: %w", err)
	}
	overlay.hwnd = hwnd
	setLayeredAttrs.Call(hwnd, 0, 235, lwaAlpha)
	corner := int32(2) // rounded on Windows 11, ignored on older systems
	if dwmSetWindowAttribute.Find() == nil {
		dwmSetWindowAttribute.Call(hwnd, 33, uintptr(unsafe.Pointer(&corner)), 4)
	}
	hdc, _, _ := getDC.Call(hwnd)
	dpi, _, _ := getDeviceCaps.Call(hdc, logPixelsY)
	releaseDC.Call(hwnd, hdc)
	overlay.scale = float64(dpi) / 96
	overlay.font = overlayFont(9, 400, dpi)
	overlay.bold = overlayFont(9, 600, dpi)

	go overlayFeed(window, home, hwnd)

	var msg winMsg
	for {
		r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return nil
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func overlayFont(points, weight int, dpi uintptr) uintptr {
	name, _ := windows.UTF16PtrFromString("Segoe UI")
	height := -points * int(dpi) / 72
	font, _, _ := createFont.Call(uintptr(int32ToPtr(height)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(name)))
	return font
}

func int32ToPtr(v int) uintptr { return uintptr(int64(v)) }

func overlayFeed(window time.Duration, home string, hwnd uintptr) {
	scanner := newSessionScanner(home)
	for {
		since := time.Now().Add(-window)
		scanner.scan(time.Time{})
		list, total := scanner.projectStatus(since)
		now := time.Now()
		footer := scanner.overlayFooter(now)
		running := runningProjects(list, scanner.openClaudeProjects(), now)
		lines := make([]overlayLine, 0, len(running)+1)
		if len(running) == 0 {
			lines = append(lines, overlayLine{Project: "Không có project đang chạy"})
		}
		for _, row := range running {
			apps := ""
			for j, app := range row.Apps {
				if j > 0 {
					apps += "+"
				}
				apps += app
			}
			ago := "đang mở"
			if !row.Last.IsZero() {
				ago = agoText(now.Sub(row.Last))
			}
			lines = append(lines, overlayLine{cutRunes(row.Project, 22), apps, ago, compactTokens(row.Total), now.Sub(row.Last) < activeWindow})
		}
		overlay.mu.Lock()
		overlay.lines, overlay.total, overlay.footer = lines, compactTokens(total.Total), footer
		overlay.mu.Unlock()
		postMessage.Call(hwnd, wmSnapshot, 0, 0)
		time.Sleep(overlayRefresh)
	}
}

func overlayProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmNcHitTest:
		return htCaption
	case wmEraseBkgnd:
		return 1
	case wmSnapshot:
		overlayResize(hwnd)
		invalidateRect.Call(hwnd, 0, 0)
		return 0
	case wmPaint:
		overlayPaint(hwnd)
		return 0
	case wmNcRButtonUp:
		overlayMenu(hwnd, lParam)
		return 0
	case wmExitSizeMove:
		overlaySavePosition(hwnd)
		return 0
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}
	r, _, _ := defWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func overlayMenu(hwnd, lParam uintptr) {
	menu, _, _ := createPopupMenu.Call()
	label, _ := windows.UTF16PtrFromString("Đóng overlay")
	appendMenu.Call(menu, 0, menuClose, uintptr(unsafe.Pointer(label)))
	setForegroundWindow.Call(hwnd)
	x, y := int32(int16(lParam&0xFFFF)), int32(int16((lParam>>16)&0xFFFF))
	choice, _, _ := trackPopupMenu.Call(menu, tpmReturnCmd, uintptr(x), uintptr(y), 0, hwnd, 0)
	destroyMenu.Call(menu)
	if choice == menuClose {
		destroyWindow.Call(hwnd)
	}
}

func textWidth(hdc uintptr, text string) (int32, int32) {
	units, _ := windows.UTF16FromString(text)
	var size winSize
	getTextExtent.Call(hdc, uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)-1), uintptr(unsafe.Pointer(&size)))
	return size.CX, size.CY
}

func drawText(hdc uintptr, x, y int32, text string, color uint32) {
	units, _ := windows.UTF16FromString(text)
	setTextColor.Call(hdc, uintptr(color))
	textOut.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)-1))
}

type overlayLayout struct {
	pad, gap, rowHeight             int32
	dot, project, apps, ago, tokens int32
	width, height                   int32
}

func (o *overlayView) px(v int) int32 { return int32(float64(v) * o.scale) }

func overlayMeasure(hdc uintptr, lines []overlayLine, title, total, footer string) overlayLayout {
	selectObject.Call(hdc, overlay.font)
	l := overlayLayout{pad: overlay.px(10), gap: overlay.px(12)}
	_, textHeight := textWidth(hdc, "Ag")
	l.rowHeight = textHeight + overlay.px(3)
	l.dot, _ = textWidth(hdc, "●  ")
	for _, line := range lines {
		for _, cell := range []struct {
			target *int32
			text   string
		}{{&l.project, line.Project}, {&l.apps, line.Apps}, {&l.ago, line.Ago}, {&l.tokens, line.Tokens}} {
			if w, _ := textWidth(hdc, cell.text); w > *cell.target {
				*cell.target = w
			}
		}
	}
	selectObject.Call(hdc, overlay.bold)
	titleWidth, _ := textWidth(hdc, title)
	totalWidth, _ := textWidth(hdc, total)
	selectObject.Call(hdc, overlay.font)
	footerWidth, _ := textWidth(hdc, footer)
	content := l.dot + l.project + l.gap + l.apps + l.gap + l.ago + l.gap + l.tokens
	if len(lines) == 0 {
		content = 0
	}
	if minimum := titleWidth + l.gap*2 + totalWidth; content < minimum {
		l.project += minimum - content
		content = minimum
	}
	if content < footerWidth {
		l.project += footerWidth - content
		content = footerWidth
	}
	rows := int32(len(lines)) + 2
	l.width = content + l.pad*2
	l.height = rows*l.rowHeight + l.pad*2
	return l
}

func overlaySnapshot() ([]overlayLine, string, string, string) {
	overlay.mu.Lock()
	defer overlay.mu.Unlock()
	return overlay.lines, overlay.title, overlay.total, overlay.footer
}

var overlayPlaced bool

func overlayResize(hwnd uintptr) {
	lines, title, total, footer := overlaySnapshot()
	hdc, _, _ := getDC.Call(hwnd)
	l := overlayMeasure(hdc, lines, title, total, footer)
	releaseDC.Call(hwnd, hdc)
	if overlayPlaced {
		setWindowPos.Call(hwnd, 0, 0, 0, uintptr(l.width), uintptr(l.height), swpNoMove|swpNoZOrder|swpNoActivate)
		return
	}
	overlayPlaced = true
	x, y := overlayStartPosition(l.width)
	setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(l.width), uintptr(l.height), swpNoZOrder|swpNoActivate)
	showWindow.Call(hwnd, swShowNoActive)
}

// overlayStartPosition restores the last spot if that monitor is still there.
func overlayStartPosition(width int32) (int32, int32) {
	if data, err := os.ReadFile(overlay.statePath); err == nil {
		var saved struct{ X, Y int32 }
		if json.Unmarshal(data, &saved) == nil {
			probe := winRect{saved.X, saved.Y, saved.X + 40, saved.Y + 40}
			if monitor, _, _ := monitorFromRect.Call(uintptr(unsafe.Pointer(&probe)), 0); monitor != 0 {
				return saved.X, saved.Y
			}
		}
	}
	screen, _, _ := getSystemMetrics.Call(0)
	margin := overlay.px(20)
	return int32(screen) - width - margin, margin
}

func overlaySavePosition(hwnd uintptr) {
	var rect winRect
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	data, err := json.Marshal(struct{ X, Y int32 }{rect.Left, rect.Top})
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(overlay.statePath), 0o755) == nil {
		_ = os.WriteFile(overlay.statePath, data, 0o644)
	}
}

func overlayPaint(hwnd uintptr) {
	var ps winPaint
	target, _, _ := beginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	defer endPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	var client winRect
	getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	// Draw off screen first so a refresh never flickers.
	hdc, _, _ := createCompatibleDC.Call(target)
	defer deleteDC.Call(hdc)
	bitmap, _, _ := createCompatibleBmp.Call(target, uintptr(client.Right), uintptr(client.Bottom))
	defer deleteObject.Call(bitmap)
	selectObject.Call(hdc, bitmap)
	brush, _, _ := createSolidBrush.Call(bgColor)
	fillRect.Call(hdc, uintptr(unsafe.Pointer(&client)), brush)
	deleteObject.Call(brush)
	setBkMode.Call(hdc, 1)

	lines, title, total, footer := overlaySnapshot()
	l := overlayMeasure(hdc, lines, title, total, footer)
	y := l.pad
	selectObject.Call(hdc, overlay.bold)
	drawText(hdc, l.pad, y, title, textColor)
	w, _ := textWidth(hdc, total)
	drawText(hdc, client.Right-l.pad-w, y, total, textColor)
	selectObject.Call(hdc, overlay.font)
	for _, line := range lines {
		y += l.rowHeight
		x := l.pad
		if line.Active {
			drawText(hdc, x, y, "●", activeColor)
		}
		x += l.dot
		drawText(hdc, x, y, line.Project, textColor)
		x += l.project + l.gap
		drawText(hdc, x, y, line.Apps, mutedColor)
		x += l.apps + l.gap
		drawText(hdc, x, y, line.Ago, mutedColor)
		w, _ := textWidth(hdc, line.Tokens)
		drawText(hdc, client.Right-l.pad-w, y, line.Tokens, textColor)
	}
	drawText(hdc, l.pad, y+l.rowHeight, footer, mutedColor)
	bitBlt.Call(target, 0, 0, uintptr(client.Right), uintptr(client.Bottom), hdc, 0, 0, 0x00CC0020)
}
