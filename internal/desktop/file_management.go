package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type DownloadFileState struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	CanRecycle bool   `json:"canRecycle"`
	Message    string `json:"message"`
}
type RecycleResult struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

var errRecycleCancelled = errors.New("已取消")

func (s *DesktopService) fileState(r DownloadRecord) DownloadFileState {
	state := DownloadFileState{ID: r.ID, Status: "unknown"}
	if r.Status != "done" {
		return state
	}
	if r.FinalPath == "" || !filepath.IsAbs(r.FinalPath) {
		state.Status = "unavailable"
		return state
	}
	info, err := os.Lstat(r.FinalPath)
	if err != nil {
		state.Status = "unavailable"
		if os.IsNotExist(err) {
			if parent, e := os.Stat(filepath.Dir(r.FinalPath)); e == nil && parent.IsDir() {
				state.Status = "missing"
			}
		}
		return state
	}
	state.Status = "unavailable"
	if !info.Mode().IsRegular() || safeLocalPath(r.FinalPath) != nil {
		return state
	}
	state.Status = "available"
	root, _ := historyDirectory()
	protected := []string{s.historyDir, root, filepath.Join(filepath.Dir(root), "credentials")}
	for _, dir := range protected {
		if dir == "" {
			continue
		}
		rel, e := filepath.Rel(dir, r.FinalPath)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return state
		}
	}
	if r.FileIdentity == "" || r.FileIdentity != fileIdentity(info) {
		state.Message = "文件身份无法确认或可能已被替换"
		return state
	}
	state.CanRecycle = true
	return state
}
func (s *DesktopService) GetDownloadFileStates(ids []string) ([]DownloadFileState, error) {
	records, err := s.DownloadHistory()
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool)
	for _, id := range ids {
		wanted[id] = true
	}
	result := []DownloadFileState{}
	for _, r := range records {
		if wanted[r.ID] {
			result = append(result, s.fileState(r))
		}
	}
	return result, nil
}
func (s *DesktopService) RecycleDownloadFiles(ids []string) ([]RecycleResult, error) {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	selected := make(map[string]bool)
	for _, id := range ids {
		selected[id] = true
	}
	records, err := s.DownloadHistory()
	if err != nil {
		return nil, err
	}
	results := []RecycleResult{}
	recycled := make(map[string]bool)
	for _, r := range records {
		if !selected[r.ID] {
			continue
		}
		delete(selected, r.ID)
		state := s.fileState(r)
		key := strings.ToLower(filepath.Clean(r.FinalPath))
		result := RecycleResult{ID: r.ID, Status: "failed", Message: state.Message}
		if result.Message == "" {
			result.Message = "文件不可用或不支持安全回收"
		}
		if state.Status == "missing" {
			result.Status = "missing"
			result.Message = "文件不存在"
		}
		if recycled[key] {
			result.Status = "recycled"
			result.Message = ""
			if err := s.removeDownloadRecord(r.ID); err != nil {
				result.Status = "history-update-failed"
				result.Message = "文件已移至回收站，记录清理失败"
			}
		} else if state.CanRecycle {
			// 同路径的其他记录必须一同被明确选中。
			shared := false
			for _, other := range records {
				if other.ID != r.ID && strings.EqualFold(filepath.Clean(other.FinalPath), filepath.Clean(r.FinalPath)) && !containsID(ids, other.ID) {
					shared = true
				}
			}
			if shared {
				result.Message = "请同时选择指向此文件的所有记录"
			} else {
				recycle := s.recycleFile
				if recycle == nil {
					recycle = recycleFile
				}
				err := recycle(r.FinalPath)
				if err != nil {
					result.Message = "无法移至回收站，文件仍保留"
					if errors.Is(err, errRecycleCancelled) {
						result.Status = "cancelled"
						result.Message = "已取消"
					}
				} else {
					recycled[key] = true
					result.Status = "recycled"
					result.Message = ""
					// 不持有 historyMu 等待 Shell，更新时重新读取最新历史。
					if err := s.removeDownloadRecord(r.ID); err != nil {
						result.Status = "history-update-failed"
						result.Message = "文件已移至回收站，记录清理失败"
					}
				}
			}
		}
		results = append(results, result)
	}
	for id := range selected {
		results = append(results, RecycleResult{ID: id, Status: "missing", Message: "记录不存在"})
	}
	return results, nil
}
func containsID(ids []string, id string) bool {
	for _, value := range ids {
		if value == id {
			return true
		}
	}
	return false
}
