package desktop

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

type fakeClipboardBackend struct {
	reads     []fakeClipboardRead
	readCalls int
	formats   []transfer.Format
	onChanged func()
}

type fakeClipboardRead struct {
	data []byte
	err  error
}

func (f *fakeClipboardBackend) ReadText() ([]byte, error) {
	idx := f.readCalls
	f.readCalls++
	if len(f.reads) == 0 {
		return nil, transfer.ErrFormat
	}
	if idx >= len(f.reads) {
		idx = len(f.reads) - 1
	}
	return f.reads[idx].data, f.reads[idx].err
}

func (f *fakeClipboardBackend) Formats() []transfer.Format {
	return f.formats
}

func (f *fakeClipboardBackend) OnChanged(fn func()) (off func()) {
	f.onChanged = fn
	return func() { f.onChanged = nil }
}

func TestReadClipboardTextClassifications(t *testing.T) {
	t.Run("ok_with_bom_and_nul", func(t *testing.T) {
		backend := &fakeClipboardBackend{
			reads: []fakeClipboardRead{{data: []byte("\uFEFFhttps://youtu.be/jNQXAC9IVRw\x00")}},
		}
		svc := &DesktopService{clipboardImpl: backend, clipboardSleep: func(time.Duration) {}}
		res := svc.ReadClipboardText()
		if res.Status != ClipboardStatusOK || res.Text != "https://youtu.be/jNQXAC9IVRw" || res.Message != "" {
			t.Fatalf("unexpected result: %+v", res)
		}
		if backend.readCalls != 1 {
			t.Fatalf("expected 1 read call, got %d", backend.readCalls)
		}
	})

	t.Run("empty_when_no_formats", func(t *testing.T) {
		backend := &fakeClipboardBackend{
			reads:   []fakeClipboardRead{{err: transfer.ErrFormat}},
			formats: nil,
		}
		svc := &DesktopService{clipboardImpl: backend, clipboardSleep: func(time.Duration) {}}
		res := svc.ReadClipboardText()
		if res.Status != ClipboardStatusEmpty || res.Text != "" {
			t.Fatalf("expected empty status, got %+v", res)
		}
	})

	t.Run("empty_when_whitespace_only", func(t *testing.T) {
		backend := &fakeClipboardBackend{
			reads: []fakeClipboardRead{{data: []byte("   \r\n\t ")}},
		}
		svc := &DesktopService{clipboardImpl: backend, clipboardSleep: func(time.Duration) {}}
		res := svc.ReadClipboardText()
		if res.Status != ClipboardStatusEmpty || res.Text != "" {
			t.Fatalf("expected empty status for whitespace text, got %+v", res)
		}
	})

	t.Run("non_text_when_image_or_files_present", func(t *testing.T) {
		backend := &fakeClipboardBackend{
			reads:   []fakeClipboardRead{{err: transfer.ErrFormat}},
			formats: []transfer.Format{transfer.PNG},
		}
		svc := &DesktopService{clipboardImpl: backend, clipboardSleep: func(time.Duration) {}}
		res := svc.ReadClipboardText()
		if res.Status != ClipboardStatusNonText || res.Text != "" {
			t.Fatalf("expected non-text status, got %+v", res)
		}
	})

	t.Run("busy_recovers_on_retry", func(t *testing.T) {
		var sleeps []time.Duration
		backend := &fakeClipboardBackend{
			reads: []fakeClipboardRead{
				{err: errors.New("mygo: the clipboard is busy")},
				{data: []byte("https://www.bilibili.com/video/BV1xx411c7mD")},
			},
		}
		svc := &DesktopService{
			clipboardImpl:  backend,
			clipboardSleep: func(d time.Duration) { sleeps = append(sleeps, d) },
		}
		res := svc.ReadClipboardText()
		if res.Status != ClipboardStatusOK || res.Text != "https://www.bilibili.com/video/BV1xx411c7mD" {
			t.Fatalf("expected recovery after busy retry, got %+v", res)
		}
		if backend.readCalls != 2 || len(sleeps) != 1 {
			t.Fatalf("expected 2 reads and 1 backoff sleep, got reads=%d sleeps=%v", backend.readCalls, sleeps)
		}
	})

	t.Run("busy_exhausted_bounded", func(t *testing.T) {
		var sleeps []time.Duration
		backend := &fakeClipboardBackend{
			reads: []fakeClipboardRead{
				{err: errors.New("mygo: the clipboard is busy")},
				{err: errors.New("mygo: the clipboard is busy")},
			},
		}
		svc := &DesktopService{
			clipboardImpl:  backend,
			clipboardSleep: func(d time.Duration) { sleeps = append(sleeps, d) },
		}
		res := svc.ReadClipboardText()
		if res.Status != ClipboardStatusBusy {
			t.Fatalf("expected busy status, got %+v", res)
		}
		if backend.readCalls != 2 || len(sleeps) != 1 {
			t.Fatalf("expected bounded 2 reads for busy error, got reads=%d sleeps=%v", backend.readCalls, sleeps)
		}
	})

	t.Run("changed_ownership_recovers_on_third_attempt", func(t *testing.T) {
		var sleeps []time.Duration
		backend := &fakeClipboardBackend{
			reads: []fakeClipboardRead{
				{err: mygo.ErrClipboardChanged},
				{err: mygo.ErrClipboardChanged},
				{data: []byte("https://demo.lofter.com/post/1234_5678")},
			},
		}
		svc := &DesktopService{
			clipboardImpl:  backend,
			clipboardSleep: func(d time.Duration) { sleeps = append(sleeps, d) },
		}
		res := svc.ReadClipboardText()
		if res.Status != ClipboardStatusOK || res.Text != "https://demo.lofter.com/post/1234_5678" {
			t.Fatalf("expected recovery after ErrClipboardChanged, got %+v", res)
		}
		if backend.readCalls != 3 || len(sleeps) != 2 {
			t.Fatalf("expected 3 reads and 2 sleeps, got reads=%d sleeps=%v", backend.readCalls, sleeps)
		}
	})

	t.Run("generic_error", func(t *testing.T) {
		backend := &fakeClipboardBackend{
			reads: []fakeClipboardRead{{err: errors.New("mygo: cannot lock clipboard data")}},
		}
		svc := &DesktopService{clipboardImpl: backend, clipboardSleep: func(time.Duration) {}}
		res := svc.ReadClipboardText()
		if res.Status != ClipboardStatusError || res.Text != "" {
			t.Fatalf("expected error status, got %+v", res)
		}
		if backend.readCalls != 1 {
			t.Fatalf("unexpected retry for non-busy error: %d", backend.readCalls)
		}
	})
}

func TestClipboardSignalStateDoesNotReadClipboard(t *testing.T) {
	backend := &fakeClipboardBackend{}
	svc := &DesktopService{clipboardImpl: backend, clipboardWatching: true}

	initial := svc.GetClipboardSignalState()
	if initial.Seq != 1 || !initial.Watching {
		t.Fatalf("unexpected initial state: %+v", initial)
	}

	svc.recordClipboardChanged()
	svc.recordClipboardChanged()

	after := svc.GetClipboardSignalState()
	if after.Seq != 3 {
		t.Fatalf("expected seq 3 after two background changes, got %d", after.Seq)
	}
	if backend.readCalls != 0 {
		t.Fatalf("background changes must never read clipboard content, got %d reads", backend.readCalls)
	}

	sig1 := svc.consumeFocusSignal()
	if sig1.Reason != "focus" || sig1.Seq != 3 || !sig1.Dirty || !sig1.Watching {
		t.Fatalf("unexpected first focus signal: %+v", sig1)
	}

	sig2 := svc.consumeFocusSignal()
	if sig2.Seq != 3 || sig2.Dirty {
		t.Fatalf("second focus without clipboard change should not be dirty: %+v", sig2)
	}
	if backend.readCalls != 0 {
		t.Fatalf("focus signal itself must not read clipboard content, got %d reads", backend.readCalls)
	}
}
