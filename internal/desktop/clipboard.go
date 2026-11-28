package desktop

import (
	"errors"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

const (
	ClipboardStatusOK      = "ok"
	ClipboardStatusEmpty   = "empty"
	ClipboardStatusNonText = "non-text"
	ClipboardStatusBusy    = "busy"
	ClipboardStatusError   = "error"
)

// ClipboardReadResult 描述桌面端原生剪贴板文本读取结果。
type ClipboardReadResult struct {
	Status  string `json:"status"`
	Text    string `json:"text"`
	Message string `json:"message"`
}

// ClipboardSignal 在窗口重新聚焦时通知前端当前剪贴板变化序号（不包含剪贴板原文）。
type ClipboardSignal struct {
	Reason   string `json:"reason"`
	Seq      uint64 `json:"seq"`
	Dirty    bool   `json:"dirty"`
	Watching bool   `json:"watching"`
}

// ClipboardSignalState 返回当前内存记录的剪贴板变化序号（不触发实际剪贴板读取）。
type ClipboardSignalState struct {
	Seq      uint64 `json:"seq"`
	Watching bool   `json:"watching"`
}

// ClipboardSignalEvent 仅传递轻量焦点与变化信号，不传递剪贴板内容。
var ClipboardSignalEvent = mygo.NewEvent[ClipboardSignal]("clipboard-signal")

// clipboardBackend 抽象剪贴板底层读写与变更监听，便于单元测试注入而不触碰宿主机真实剪贴板。
type clipboardBackend interface {
	ReadText() ([]byte, error)
	Formats() []transfer.Format
	OnChanged(fn func()) (off func())
}

type mygoClipboardBackend struct{}

func (mygoClipboardBackend) ReadText() ([]byte, error) {
	return mygo.Clipboard.ReadFormat(transfer.Text)
}

func (mygoClipboardBackend) Formats() []transfer.Format {
	return mygo.Clipboard.Formats()
}

func (mygoClipboardBackend) OnChanged(fn func()) (off func()) {
	return mygo.Clipboard.OnChanged(fn)
}

func (s *DesktopService) clipboard() clipboardBackend {
	s.clipboardMu.Lock()
	defer s.clipboardMu.Unlock()
	if s.clipboardImpl != nil {
		return s.clipboardImpl
	}
	return mygoClipboardBackend{}
}

func (s *DesktopService) sleepClipboard(d time.Duration) {
	s.clipboardMu.Lock()
	fn := s.clipboardSleep
	s.clipboardMu.Unlock()
	if fn != nil {
		fn(d)
		return
	}
	time.Sleep(d)
}

func isClipboardBusyError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, mygo.ErrClipboardChanged) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "clipboard is busy") || strings.Contains(msg, "clipbrd_e_cant_open")
}

func normalizeClipboardText(raw []byte) string {
	text := string(raw)
	text = strings.TrimPrefix(text, "\uFEFF")
	text = strings.TrimRight(text, "\x00")
	return text
}

// ReadClipboardText 使用 MyGo 原生剪贴板能力读取纯文本内容。
// 区分空剪贴板、非文本内容、暂时占用与读取失败，并对短暂占用做有界重试而不阻塞主线程。
func (s *DesktopService) ReadClipboardText() ClipboardReadResult {
	backend := s.clipboard()

	var (
		raw []byte
		err error
	)
	for attempt := 0; attempt < 3; attempt++ {
		raw, err = backend.ReadText()
		if err == nil || !isClipboardBusyError(err) {
			break
		}
		// MyGo Windows 底层对 "clipboard is busy" 已执行约 1s 消息泵等待，因此仅额外补试 1 次；
		// 若为瞬时所有权切换（ErrClipboardChanged），允许最多补试 2 次。
		if !errors.Is(err, mygo.ErrClipboardChanged) && attempt >= 1 {
			break
		}
		if attempt < 2 {
			backoff := 40 * time.Millisecond
			if attempt == 1 {
				backoff = 80 * time.Millisecond
			}
			s.sleepClipboard(backoff)
		}
	}

	if err == nil {
		text := normalizeClipboardText(raw)
		if strings.TrimSpace(text) == "" {
			return ClipboardReadResult{
				Status:  ClipboardStatusEmpty,
				Message: "剪贴板为空",
			}
		}
		return ClipboardReadResult{
			Status: ClipboardStatusOK,
			Text:   text,
		}
	}

	if errors.Is(err, transfer.ErrFormat) {
		formats := backend.Formats()
		if len(formats) == 0 {
			return ClipboardReadResult{
				Status:  ClipboardStatusEmpty,
				Message: "剪贴板为空",
			}
		}
		return ClipboardReadResult{
			Status:  ClipboardStatusNonText,
			Message: "剪贴板当前不是文本内容",
		}
	}

	if isClipboardBusyError(err) {
		return ClipboardReadResult{
			Status:  ClipboardStatusBusy,
			Message: "剪贴板暂时被其他程序占用，请稍后重试或按 Ctrl+V 粘贴",
		}
	}

	return ClipboardReadResult{
		Status:  ClipboardStatusError,
		Message: "无法读取剪贴板，请使用 Ctrl+V 手动粘贴",
	}
}

// GetClipboardSignalState 返回当前已观测到的剪贴板变化序号，绝不读取剪贴板正文。
func (s *DesktopService) GetClipboardSignalState() ClipboardSignalState {
	s.clipboardMu.Lock()
	defer s.clipboardMu.Unlock()
	if s.clipboardSeq == 0 {
		s.clipboardSeq = 1
	}
	return ClipboardSignalState{
		Seq:      s.clipboardSeq,
		Watching: s.clipboardWatching,
	}
}

func (s *DesktopService) recordClipboardChanged() {
	s.clipboardMu.Lock()
	if s.clipboardSeq == 0 {
		s.clipboardSeq = 1
	}
	s.clipboardSeq++
	s.clipboardDirty = true
	s.clipboardMu.Unlock()
}

func (s *DesktopService) consumeFocusSignal() ClipboardSignal {
	s.clipboardMu.Lock()
	if s.clipboardSeq == 0 {
		s.clipboardSeq = 1
	}
	sig := ClipboardSignal{
		Reason:   "focus",
		Seq:      s.clipboardSeq,
		Dirty:    s.clipboardDirty,
		Watching: s.clipboardWatching,
	}
	s.clipboardDirty = false
	s.clipboardMu.Unlock()
	return sig
}

func (s *DesktopService) stopClipboardWatch() {
	s.clipboardMu.Lock()
	offs := s.clipboardOff
	s.clipboardOff = nil
	s.clipboardWatching = false
	s.clipboardMu.Unlock()
	for _, off := range offs {
		if off != nil {
			off()
		}
	}
}

func (s *DesktopService) setupClipboardWatch(w *mygo.Window) {
	s.stopClipboardWatch()
	if w == nil {
		return
	}

	backend := s.clipboard()
	s.clipboardMu.Lock()
	if s.clipboardSeq == 0 {
		s.clipboardSeq = 1
	}
	s.clipboardWatching = true
	s.clipboardMu.Unlock()

	offChanged := backend.OnChanged(func() {
		// 后台仅记录变化序号，严禁在此读取剪贴板内容。
		s.recordClipboardChanged()
	})
	offFocus := w.OnFocus(func() {
		sig := s.consumeFocusSignal()
		if s.win == w {
			ClipboardSignalEvent.Emit(w, sig)
		}
	})
	offClosed := w.OnClosed(func() {
		s.stopClipboardWatch()
	})

	s.clipboardMu.Lock()
	s.clipboardOff = []func(){offChanged, offFocus, offClosed}
	s.clipboardMu.Unlock()
}
