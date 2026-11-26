package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileManagement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(t.TempDir(), "中文 media.mp4")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	r := DownloadRecord{ID: "one", Status: "done", FinalPath: path, FileIdentity: fileIdentity(info)}
	s := &DesktopService{historyDir: dir}
	if err := writeHistory(dir, []DownloadRecord{r}); err != nil {
		t.Fatal(err)
	}
	states, err := s.GetDownloadFileStates([]string{"one"})
	if err != nil || len(states) != 1 || states[0].Status != "available" || !states[0].CanRecycle {
		t.Fatalf("%+v %v", states, err)
	}
	s.recycleFile = func(string) error { return errRecycleCancelled }
	results, err := executeTestDeletion(t, s, []string{"one"})
	if err != nil || results[0].Status != "cancelled" {
		t.Fatalf("%+v %v", results, err)
	}
	records, _ := s.DownloadHistory()
	if len(records) != 1 {
		t.Fatal("cancel cleared history")
	}
	// 模拟 Shell 成功，只移动隔离目录中的文件，不访问系统回收站。
	s.recycleFile = func(path string) error { return os.Rename(path, path+".mock-recycled") }
	results, err = executeTestDeletion(t, s, []string{"one"})
	if err != nil || results[0].Status != "recycled" {
		t.Fatalf("%+v %v", results, err)
	}
	records, _ = s.DownloadHistory()
	if len(records) != 0 {
		t.Fatal("history retained")
	}
	if _, err := os.Stat(path + ".mock-recycled"); err != nil {
		t.Fatal(err)
	}
}

func TestFileIdentityAndInvalidPaths(t *testing.T) {
	s := &DesktopService{historyDir: t.TempDir()}
	path := filepath.Join(t.TempDir(), "media")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	r := DownloadRecord{ID: "one", Status: "done", FinalPath: path, FileIdentity: fileIdentity(info)}
	if err := os.WriteFile(path, []byte("different"), 0600); err != nil {
		t.Fatal(err)
	}
	if state := s.fileState(r); state.CanRecycle {
		t.Fatal("replacement accepted")
	}
	r.FileIdentity = ""
	if s.fileState(r).CanRecycle {
		t.Fatal("missing identity must not be accepted")
	}
	for _, path := range []string{"", ".", t.TempDir()} {
		r.FinalPath = path
		if s.fileState(r).CanRecycle {
			t.Fatal("invalid target accepted")
		}
	}
	r.FinalPath = filepath.Join(t.TempDir(), "missing")
	if s.fileState(r).Status != "missing" {
		t.Fatal("missing file not identified")
	}
	s.recycleFile = func(string) error { return errors.New("must not call") }
	r.Status = "running"
	if err := writeHistory(s.historyDir, []DownloadRecord{r}); err != nil {
		t.Fatal(err)
	}
	results, err := executeTestDeletion(t, s, []string{r.ID})
	if err != nil || len(results) > 0 && results[0].Status == "recycled" {
		t.Fatalf("%+v %v", results, err)
	}
}

func TestRecyclePartialAndDuplicateTargets(t *testing.T) {
	dir := t.TempDir()
	root := t.TempDir()
	makeRecord := func(id string) DownloadRecord {
		path := filepath.Join(root, id+".mp4")
		if err := os.WriteFile(path, []byte(id), 0600); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		return DownloadRecord{ID: id, Status: "done", FinalPath: path, FileIdentity: fileIdentity(info)}
	}
	one, two := makeRecord("one"), makeRecord("two")
	duplicate := one
	duplicate.ID = "duplicate"
	s := &DesktopService{historyDir: dir}
	if err := writeHistory(dir, []DownloadRecord{one, duplicate, two}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.recycleFile = func(path string) error {
		calls++
		if path == two.FinalPath {
			return errors.New("unavailable")
		}
		// 并发新增历史不得被最终清理覆盖。
		s.historyMu.Lock()
		latest, _ := readHistory(dir)
		err := writeHistory(dir, append(latest, DownloadRecord{ID: "new", Status: "done"}))
		s.historyMu.Unlock()
		if err != nil {
			return err
		}
		return os.Rename(path, path+".mock")
	}
	result, err := executeTestDeletion(t, s, []string{"one", "duplicate", "two"})
	if err != nil || len(result) != 3 || calls != 2 {
		t.Fatalf("%+v %v calls=%d", result, err, calls)
	}
	records, _ := s.DownloadHistory()
	if len(records) != 2 {
		t.Fatalf("unexpected records %+v", records)
	}
	for _, r := range records {
		if r.ID != "two" && r.ID != "new" {
			t.Fatalf("%+v", records)
		}
	}
}

// 测试同样通过实际公开计划接口执行，不保留绕过确认的入口。
func executeTestDeletion(t *testing.T, s *DesktopService, ids []string) ([]RecycleResult, error) {
	t.Helper()
	plan, err := s.PrepareDownloadDeletion(ids)
	if err != nil {
		return nil, err
	}
	return s.ExecuteDownloadDeletion(plan.Token)
}
