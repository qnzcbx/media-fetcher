package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeletePlanIdentity(t *testing.T) {
	s := &DesktopService{historyDir: t.TempDir()}
	path := filepath.Join(t.TempDir(), "media.mp4")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeHistory(s.historyDir, []DownloadRecord{{ID: "one", Status: "done", FinalPath: path, FileIdentity: fileIdentity(mustStat(t, path))}}); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PrepareDownloadDeletion([]string{"one"})
	if err != nil || len(plan.Targets) != 1 {
		t.Fatalf("%+v %v", plan, err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecuteDownloadDeletion(plan.Token); err == nil {
		t.Fatal("确认期间替换被接受")
	}
	if _, err := s.ExecuteDownloadDeletion(plan.Token); err == nil {
		t.Fatal("重复计划被接受")
	}
	records, _ := s.DownloadHistory()
	records[0].FileIdentity = fileIdentity(mustStat(t, path))
	if err := writeHistory(s.historyDir, records); err != nil {
		t.Fatal(err)
	}
	plan, err = s.PrepareDownloadDeletion([]string{"one"})
	if err != nil {
		t.Fatal(err)
	}
	s.recycleFile = func(path string) error { return os.Rename(path, path+".mock") }
	results, err := s.ExecuteDownloadDeletion(plan.Token)
	if err != nil || len(results) != 1 || results[0].Status != "recycled" {
		t.Fatalf("%+v %v", results, err)
	}
}

func TestExpiredDeletePlan(t *testing.T) {
	s := &DesktopService{historyDir: t.TempDir(), deletePlans: map[string]deleteSnapshot{"expired": {expires: time.Now().Add(-time.Second)}}}
	if _, err := s.ExecuteDownloadDeletion("expired"); err == nil {
		t.Fatal("过期计划被接受")
	}
}
func TestOverallTrackProgress(t *testing.T) {
	p := trackProgress{total: [2]int64{180, 20}}
	if n, total := p.update(0, 180, 180); n != 180 || total != 200 {
		t.Fatal(n, total)
	}
	if n, total := p.update(1, 0, 20); n != 180 || total != 200 {
		t.Fatal(n, total)
	}
	if n, total := p.update(1, 20, 20); n != 200 || total != 200 {
		t.Fatal(n, total)
	}
	if n, total := p.update(1, 0, 0); n != 0 || total != 0 {
		t.Fatal(n, total)
	}
	if n, total := p.update(1, 5, 20); n != 185 || total != 200 {
		t.Fatal(n, total)
	}
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
