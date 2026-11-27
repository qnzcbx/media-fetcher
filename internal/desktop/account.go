package desktop

import "errors"

// useAccount 与退出接口互斥登记，覆盖解析和下载的整个生命周期。
func (s *DesktopService) useAccount(siteID, accountID string) func() {
	if accountID == "" {
		return func() {}
	}
	key := siteID + ":" + accountID
	s.accountMu.Lock()
	if s.accountUses == nil {
		s.accountUses = make(map[string]int)
	}
	s.accountUses[key]++
	s.accountMu.Unlock()
	return func() {
		s.accountMu.Lock()
		defer s.accountMu.Unlock()
		s.accountUses[key]--
		if s.accountUses[key] == 0 {
			delete(s.accountUses, key)
		}
	}
}

// LogoutAccount 仅清理 Media Fetcher 本地指定账号，不触发平台远程注销。
func (s *DesktopService) LogoutAccount(siteID, accountID string) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if siteID == "" || accountID == "" {
		return errors.New("请选择要退出的账号")
	}
	if s.accountUses[siteID+":"+accountID] > 0 {
		return errors.New("该账号正在解析或下载，请等待任务结束后退出")
	}
	if err := s.credService.DeleteAccount(siteID, accountID); err != nil {
		return errors.New("退出失败，未能完整清理本地凭证，请重试")
	}
	return nil
}
