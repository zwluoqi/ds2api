package client

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
	trans "ds2api/internal/deepseek/transport"
)

type requestClients struct {
	regular   trans.Doer
	stream    trans.Doer
	fallback  *http.Client
	fallbackS *http.Client
}

type hostLookupFunc func(ctx context.Context, network, host string) ([]string, error)

var proxyConnectivityTestURL = "https://chat.deepseek.com/"

var defaultHostLookup hostLookupFunc = func(ctx context.Context, _ string, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

func proxyDialAddress(ctx context.Context, proxyType, address string, lookup hostLookupFunc) (string, error) {
	proxyType = strings.ToLower(strings.TrimSpace(proxyType))
	if proxyType != "socks5" {
		return address, nil
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if net.ParseIP(host) != nil {
		return address, nil
	}
	if lookup == nil {
		lookup = defaultHostLookup
	}
	addrs, err := lookup(ctx, "ip", host)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("no ip address resolved for %s", host)
	}
	return net.JoinHostPort(addrs[0], port), nil
}

func proxyCacheKey(proxyCfg config.Proxy) string {
	proxyCfg = config.NormalizeProxy(proxyCfg)
	return strings.Join([]string{
		proxyCfg.ID,
		proxyCfg.Type,
		strings.ToLower(proxyCfg.Host),
		strconv.Itoa(proxyCfg.Port),
		proxyCfg.Username,
		proxyCfg.Password,
	}, "|")
}

func proxyDialContext(proxyCfg config.Proxy) (trans.DialContextFunc, error) {
	proxyCfg = config.NormalizeProxy(proxyCfg)
	if proxyCfg.Type == "http" {
		return httpProxyDialContext(proxyCfg), nil
	}
	var authCfg *proxy.Auth
	if proxyCfg.Username != "" || proxyCfg.Password != "" {
		authCfg = &proxy.Auth{User: proxyCfg.Username, Password: proxyCfg.Password}
	}
	forward := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	dialer, err := proxy.SOCKS5("tcp", net.JoinHostPort(proxyCfg.Host, strconv.Itoa(proxyCfg.Port)), authCfg, forward)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		target, err := proxyDialAddress(ctx, proxyCfg.Type, address, defaultHostLookup)
		if err != nil {
			return nil, err
		}
		if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
			return ctxDialer.DialContext(ctx, network, target)
		}
		return dialer.Dial(network, target)
	}, nil
}

func httpProxyDialContext(proxyCfg config.Proxy) trans.DialContextFunc {
	proxyAddr := net.JoinHostPort(proxyCfg.Host, strconv.Itoa(proxyCfg.Port))
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, proxyAddr)
		if err != nil {
			return nil, err
		}
		if deadline, ok := ctx.Deadline(); ok {
			if err := conn.SetDeadline(deadline); err != nil {
				closeErr := conn.Close()
				return nil, fmt.Errorf("set proxy deadline: %w", errors.Join(err, closeErr))
			}
		}
		if err := writeHTTPProxyConnect(conn, proxyCfg, address); err != nil {
			closeErr := conn.Close()
			return nil, errors.Join(err, closeErr)
		}
		resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
		if err != nil {
			closeErr := conn.Close()
			return nil, errors.Join(err, closeErr)
		}
		if resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				closeErr := conn.Close()
				return nil, errors.Join(err, closeErr)
			}
		}
		if resp.StatusCode != http.StatusOK {
			closeErr := conn.Close()
			return nil, errors.Join(fmt.Errorf("http proxy CONNECT failed: %s", resp.Status), closeErr)
		}
		if err := conn.SetDeadline(time.Time{}); err != nil {
			closeErr := conn.Close()
			return nil, fmt.Errorf("clear proxy deadline: %w", errors.Join(err, closeErr))
		}
		return conn, nil
	}
}

func writeHTTPProxyConnect(conn net.Conn, proxyCfg config.Proxy, address string) error {
	var b strings.Builder
	b.WriteString("CONNECT ")
	b.WriteString(address)
	b.WriteString(" HTTP/1.1\r\nHost: ")
	b.WriteString(address)
	b.WriteString("\r\nProxy-Connection: Keep-Alive\r\n")
	if proxyCfg.Username != "" || proxyCfg.Password != "" {
		authValue := base64.StdEncoding.EncodeToString([]byte(proxyCfg.Username + ":" + proxyCfg.Password))
		b.WriteString("Proxy-Authorization: Basic ")
		b.WriteString(authValue)
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n")
	_, err := conn.Write([]byte(b.String()))
	return err
}

func (c *Client) defaultRequestClients() requestClients {
	return requestClients{
		regular:   c.regular,
		stream:    c.stream,
		fallback:  c.fallback,
		fallbackS: c.fallbackS,
	}
}

func (c *Client) resolveProxyForAccount(acc config.Account) (config.Proxy, bool) {
	if c == nil || c.Store == nil {
		return config.Proxy{}, false
	}
	proxyID := strings.TrimSpace(acc.ProxyID)
	if proxyID == "" {
		return config.Proxy{}, false
	}
	snap := c.Store.Snapshot()
	for _, proxyCfg := range snap.Proxies {
		proxyCfg = config.NormalizeProxy(proxyCfg)
		if proxyCfg.ID == proxyID {
			return proxyCfg, true
		}
	}
	return config.Proxy{}, false
}

func (c *Client) requestClientsFromContext(ctx context.Context) requestClients {
	if a, ok := auth.FromContext(ctx); ok {
		return c.requestClientsForAccount(a.Account)
	}
	return c.defaultRequestClients()
}

func (c *Client) requestClientsForAuth(ctx context.Context, a *auth.RequestAuth) requestClients {
	if a != nil {
		return c.requestClientsForAccount(a.Account)
	}
	return c.requestClientsFromContext(ctx)
}

func (c *Client) requestClientsForAccount(acc config.Account) requestClients {
	proxyCfg, ok := c.resolveProxyForAccount(acc)
	if !ok {
		return c.defaultRequestClients()
	}

	key := proxyCacheKey(proxyCfg)
	c.proxyClientsMu.RLock()
	cached, ok := c.proxyClients[key]
	c.proxyClientsMu.RUnlock()
	if ok {
		return cached
	}

	dialContext, err := proxyDialContext(proxyCfg)
	if err != nil {
		config.Logger.Warn("[proxy] build dialer failed", "proxy_id", proxyCfg.ID, "error", err)
		return c.defaultRequestClients()
	}

	bundle := requestClients{
		regular:   trans.NewWithDialContext(60*time.Second, dialContext),
		stream:    trans.NewWithDialContext(0, dialContext),
		fallback:  trans.NewFallbackClient(60*time.Second, dialContext),
		fallbackS: trans.NewFallbackClient(0, dialContext),
	}

	c.proxyClientsMu.Lock()
	if c.proxyClients == nil {
		c.proxyClients = make(map[string]requestClients)
	}
	c.proxyClients[key] = bundle
	c.proxyClientsMu.Unlock()
	return bundle
}

func applyProxyConnectivityHeaders(req *http.Request) {
	if req == nil {
		return
	}
	for key, value := range dsprotocol.BaseHeaders {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		req.Header.Set(key, value)
	}
}

func proxyConnectivityStatus(statusCode int) (bool, string) {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return true, fmt.Sprintf("代理可达，目标返回 HTTP %d", statusCode)
	case statusCode >= 300 && statusCode < 500:
		return true, fmt.Sprintf("代理可达，但目标返回 HTTP %d（可能是风控或挑战）", statusCode)
	default:
		return false, fmt.Sprintf("目标返回 HTTP %d", statusCode)
	}
}

func TestProxyConnectivity(ctx context.Context, proxyCfg config.Proxy) map[string]any {
	start := time.Now()
	proxyCfg = config.NormalizeProxy(proxyCfg)
	result := map[string]any{
		"success":       false,
		"proxy_id":      proxyCfg.ID,
		"proxy_type":    proxyCfg.Type,
		"response_time": 0,
	}

	if err := config.ValidateProxyConfig([]config.Proxy{proxyCfg}); err != nil {
		result["message"] = "代理配置无效: " + err.Error()
		return result
	}
	dialContext, err := proxyDialContext(proxyCfg)
	if err != nil {
		result["message"] = "代理拨号器初始化失败: " + err.Error()
		return result
	}

	client := trans.NewFallbackClient(15*time.Second, dialContext)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyConnectivityTestURL, nil)
	if err != nil {
		result["message"] = err.Error()
		return result
	}
	applyProxyConnectivityHeaders(req)

	resp, err := client.Do(req)
	result["response_time"] = int(time.Since(start).Milliseconds())
	if err != nil {
		result["message"] = err.Error()
		return result
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			config.Logger.Warn("[proxy] close response body failed", "proxy_id", proxyCfg.ID, "error", closeErr)
		}
	}()

	result["status_code"] = resp.StatusCode
	result["success"], result["message"] = proxyConnectivityStatus(resp.StatusCode)
	return result
}
