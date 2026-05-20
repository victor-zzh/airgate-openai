package gateway

import "testing"

func TestRedactProxyURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "no auth", in: "http://10.0.0.1:7890", want: "http://10.0.0.1:7890"},
		{name: "with user only", in: "http://user@10.0.0.1:7890", want: "http://10.0.0.1:7890"},
		{name: "with user:pass", in: "http://user:secret@10.0.0.1:7890", want: "http://10.0.0.1:7890"},
		{name: "socks5 with auth", in: "socks5://u:p@1.2.3.4:1080", want: "socks5://1.2.3.4:1080"},
		{name: "https proxy with auth", in: "https://admin:pwd123@proxy.example.com:8443", want: "https://proxy.example.com:8443"},
		{name: "malformed", in: "://bad", want: "<invalid>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactProxyURL(tc.in); got != tc.want {
				t.Errorf("redactProxyURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
