package client

import (
	"bufio"
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
)

func TestProxyDialAddressUsesLocalResolutionForSocks5(t *testing.T) {
	ctx := context.Background()
	resolved, err := proxyDialAddress(ctx, "socks5", "example.com:443", func(_ context.Context, network, host string) ([]string, error) {
		if network != "ip" {
			t.Fatalf("unexpected lookup network: %q", network)
		}
		if host != "example.com" {
			t.Fatalf("unexpected lookup host: %q", host)
		}
		return []string{"203.0.113.10"}, nil
	})
	if err != nil {
		t.Fatalf("proxyDialAddress returned error: %v", err)
	}
	if resolved != "203.0.113.10:443" {
		t.Fatalf("expected locally resolved address, got %q", resolved)
	}
}

func TestProxyDialAddressKeepsHostnameForSocks5h(t *testing.T) {
	ctx := context.Background()
	lookups := 0
	resolved, err := proxyDialAddress(ctx, "socks5h", "example.com:443", func(_ context.Context, network, host string) ([]string, error) {
		lookups++
		return []string{"203.0.113.10"}, nil
	})
	if err != nil {
		t.Fatalf("proxyDialAddress returned error: %v", err)
	}
	if resolved != "example.com:443" {
		t.Fatalf("expected hostname preserved for remote DNS, got %q", resolved)
	}
	if lookups != 0 {
		t.Fatalf("expected no local DNS lookup for socks5h, got %d", lookups)
	}
}

func TestHTTPProxyDialContextUsesConnectAndAuth(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		if err := ln.Close(); err != nil {
			t.Fatalf("close listener: %v", err)
		}
	}()

	got := make(chan *http.Request, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() {
			if err := conn.Close(); err != nil {
				t.Errorf("close proxy conn: %v", err)
			}
		}()
		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			return
		}
		got <- req
		if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			return
		}
	}()

	host, portText, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split listener addr: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse listener port: %v", err)
	}
	dialContext, err := proxyDialContext(config.Proxy{Type: "http", Host: host, Port: port, Username: "user", Password: "pass"})
	if err != nil {
		t.Fatalf("proxyDialContext: %v", err)
	}
	conn, err := dialContext(context.Background(), "tcp", "chat.deepseek.com:443")
	if err != nil {
		t.Fatalf("dial via http proxy: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close tunneled conn: %v", err)
	}
	<-done

	select {
	case req := <-got:
		if req.Method != http.MethodConnect || req.Host != "chat.deepseek.com:443" {
			t.Fatalf("unexpected CONNECT request: method=%q host=%q", req.Method, req.Host)
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		if req.Header.Get("Proxy-Authorization") != wantAuth {
			t.Fatalf("expected proxy auth %q, got %q", wantAuth, req.Header.Get("Proxy-Authorization"))
		}
	default:
		t.Fatal("proxy did not receive CONNECT request")
	}
}

func TestApplyProxyConnectivityHeadersUsesBaseHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://chat.deepseek.com/", nil)
	if err != nil {
		t.Fatalf("http.NewRequest returned error: %v", err)
	}

	applyProxyConnectivityHeaders(req)

	for key, want := range dsprotocol.BaseHeaders {
		if got := req.Header.Get(key); got != want {
			t.Fatalf("expected header %q=%q, got %q", key, want, got)
		}
	}
}

func TestProxyConnectivityStatus(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		success    bool
		wantText   string
	}{
		{name: "ok", statusCode: 200, success: true, wantText: "HTTP 200"},
		{name: "challenge", statusCode: 403, success: true, wantText: "风控或挑战"},
		{name: "upstream error", statusCode: 502, success: false, wantText: "HTTP 502"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			success, message := proxyConnectivityStatus(tc.statusCode)
			if success != tc.success {
				t.Fatalf("expected success=%v, got %v", tc.success, success)
			}
			if message == "" || !strings.Contains(message, tc.wantText) {
				t.Fatalf("expected message to contain %q, got %q", tc.wantText, message)
			}
		})
	}
}
