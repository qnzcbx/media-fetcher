package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLegacyDeletePlan(t *testing.T) {
	s := &DesktopService{historyDir: t.TempDir()}
	path := filepath.Join(t.TempDir(), "legacy.mp4")
	if err := os.WriteFile(path, []byte("legacy"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeHistory(s.historyDir, []DownloadRecord{{ID: "old", Status: "done", FinalPath: path}}); err != nil {
		t.Fatal(err)
	}
	plan, err := s.PrepareDownloadDeletion([]string{"old"})
	if err != nil || len(plan.Targets) != 1 || !plan.Targets[0].Legacy {
		t.Fatalf("%+v %v", plan, err)
	}
	records, _ := s.DownloadHistory()
	if records[0].FileIdentity != "" {
		t.Fatal("旧身份被伪造")
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
	plan, err = s.PrepareDownloadDeletion([]string{"old"})
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
