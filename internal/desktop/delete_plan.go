package desktop

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"
)

type DeleteTarget struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Legacy bool   `json:"legacy"`
}
type DeletePlan struct {
	Token   string         `json:"token"`
	Targets []DeleteTarget `json:"targets"`
	Skipped int            `json:"skipped"`
}
type deleteSnapshot struct {
	expires    time.Time
	identities map[string]string
	records    map[string]DownloadRecord
}

func (s *DesktopService) PrepareDownloadDeletion(ids []string) (DeletePlan, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	records, err := s.DownloadHistory()
	if err != nil {
		return DeletePlan{}, err
	}
	if s.deletePlans == nil {
		s.deletePlans = make(map[string]deleteSnapshot)
	}
	for token, plan := range s.deletePlans {
		if time.Now().After(plan.expires) {
			delete(s.deletePlans, token)
		}
	}
	clear(s.deletePlans)
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return DeletePlan{}, err
	}
	plan := DeletePlan{Token: hex.EncodeToString(bytes), Targets: []DeleteTarget{}}
	snapshot := deleteSnapshot{expires: time.Now().Add(2 * time.Minute), identities: map[string]string{}, records: map[string]DownloadRecord{}}
	for _, r := range records {
		if !containsID(ids, r.ID) {
			continue
		}
		check := r
		legacy := r.FileIdentity == ""
		info, err := os.Lstat(r.FinalPath)
		if err == nil && legacy {
			check.FileIdentity = fileIdentity(info)
		}
		if err != nil || !s.fileState(check).CanRecycle {
			continue
		}
		plan.Targets = append(plan.Targets, DeleteTarget{ID: r.ID, Name: info.Name(), Path: r.FinalPath, Legacy: legacy})
		snapshot.identities[r.ID] = fileIdentity(info)
		snapshot.records[r.ID] = r
	}
	plan.Skipped = len(ids) - len(plan.Targets)
	s.deletePlans[plan.Token] = snapshot
	return plan, nil
}

func (s *DesktopService) ExecuteDownloadDeletion(token string) ([]RecycleResult, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	plan, ok := s.deletePlans[token]
	delete(s.deletePlans, token)
	if !ok || time.Now().After(plan.expires) {
		return nil, errors.New("删除确认已过期，请重新操作")
	}
	records, err := s.DownloadHistory()
	if err != nil {
		return nil, err
	}
	checked := map[string]string{}
	ids := []string{}
	for _, r := range records {
		expected, selected := plan.identities[r.ID]
		if !selected {
			continue
		}
		original := plan.records[r.ID]
		info, err := os.Lstat(r.FinalPath)
		if err != nil || r.FinalPath != original.FinalPath || r.FileIdentity != original.FileIdentity || fileIdentity(info) != expected {
			return nil, errors.New("文件在确认期间发生变化，请重新检查")
		}
		if r.FileIdentity == "" {
			r.FileIdentity = expected
		}
		if !s.fileState(r).CanRecycle {
			return nil, errors.New("文件已不可安全回收")
		}
		checked[r.ID] = expected
		ids = append(ids, r.ID)
	}
	if len(ids) != len(plan.identities) {
		return nil, errors.New("下载记录已变化，请重新操作")
	}
	return s.recycleDownloadFiles(ids, checked)
}
