package desktop

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"media-fetcher/core/auth"
	"media-fetcher/core/extractor"
	"media-fetcher/pkg/cookiejar"
	"media-fetcher/pkg/vault"
)

type logoutBlockedExtractor struct{ started, release chan struct{} }

func (logoutBlockedExtractor) SiteID() string    { return "bilibili" }
func (logoutBlockedExtractor) Match(string) bool { return true }
func (e logoutBlockedExtractor) Extract(context.Context, string, extractor.ExtractOptions) (*extractor.MediaInfo, error) {
	close(e.started)
	<-e.release
	return nil, errors.New("fixture complete")
}

func TestLogoutAccountLifecycle(t *testing.T) {
	cred, err := auth.NewCredentialService(t.TempDir(), vault.NewMockVault("logout"))
	if err != nil {
		t.Fatal(err)
	}
	jar := cookiejar.New("bilibili", "account")
	u, _ := url.Parse("https://api.bilibili.com")
	jar.SetCookies(u, []*http.Cookie{{Name: "SESSDATA", Value: "fixture", Path: "/"}})
	if err := cred.RegisterOrUpdateAccount(&auth.Account{SiteID: "bilibili", AccountID: "account", Name: "fixture"}, jar); err != nil {
		t.Fatal(err)
	}
	client, err := cred.GetClient("bilibili", "account")
	if err != nil {
		t.Fatal(err)
	}
	s := &DesktopService{credService: cred, registry: extractor.NewRegistry()}
	e := logoutBlockedExtractor{make(chan struct{}), make(chan struct{})}
	s.registry.Register(e)
	done := make(chan struct{})
	go func() { defer close(done); _, _ = s.Extract("fixture", "account") }()
	<-e.started
	if s.LogoutAccount("bilibili", "account") == nil {
		t.Fatal("解析期间允许退出")
	}
	close(e.release)
	<-done
	// 下载登记同样保护指定账号；其他账号的使用不会阻止退出。
	finish := s.useAccount("bilibili", "account")
	if s.LogoutAccount("bilibili", "account") == nil {
		t.Fatal("下载期间允许退出")
	}
	finish()
	other := s.useAccount("bilibili", "other")
	defer other()
	if err := s.LogoutAccount("bilibili", "account"); err != nil {
		t.Fatal(err)
	}
	if _, err := cred.GetClient("bilibili", "account"); err == nil {
		t.Fatal("退出后仍能获取账号")
	}
	if len(client.Jar.Cookies(u)) != 0 {
		t.Fatal("旧 CookieJar 未撤销")
	}
	if len(s.GetAccounts()) != 0 {
		t.Fatal("退出后仍展示账号")
	}
	if s.LogoutAccount("bilibili", "account") == nil {
		t.Fatal("不存在账号不得虚报退出成功")
	}
}
