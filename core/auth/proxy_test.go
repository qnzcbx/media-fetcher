package auth

import (
	"net/http"
	"testing"
)

func TestParseSystemProxyServer(t *testing.T) {
	cases := []struct {
		name   string
		server string
		scheme string
		want   string
	}{
		{"single_host_port", "127.0.0.1:7890", "https", "http://127.0.0.1:7890"},
		{"single_with_scheme", "http://127.0.0.1:7890", "https", "http://127.0.0.1:7890"},
		{"per_scheme_https", "http=127.0.0.1:7890;https=127.0.0.1:7891", "https", "http://127.0.0.1:7891"},
		{"per_scheme_fallback_http", "http=127.0.0.1:7890", "https", "http://127.0.0.1:7890"},
		{"per_scheme_socks", "socks=127.0.0.1:7892", "https", "socks5://127.0.0.1:7892"},
		{"empty", "", "https", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := parseSystemProxyServer(tc.server, tc.scheme)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			got := ""
			if u != nil {
				got = u.String()
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShouldBypassSystemProxy(t *testing.T) {
	override := "localhost;127.*;*.bilibili.com;192.168.*;<local>"
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"api.bilibili.com", true},
		{"192.168.1.10", true},
		{"intranet", true},
		{"www.youtube.com", false},
		{"rr1---sn-test.googlevideo.com", false},
	}
	for _, tc := range cases {
		if got := shouldBypassSystemProxy(tc.host, override); got != tc.want {
			t.Errorf("host=%q got=%v want=%v", tc.host, got, tc.want)
		}
	}
}

func TestProxyFromEnvTakesPrecedence(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:18080")
	req, err := http.NewRequest(http.MethodGet, "https://www.youtube.com/watch?v=aqz-KE-bpKQ", nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := proxyFromEnvOrSystem(req)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || u.String() != "http://127.0.0.1:18080" {
		t.Fatalf("expected env proxy http://127.0.0.1:18080, got %v", u)
	}
}
