// Package httpclient builds outbound HTTP clients with optional proxy layers.
package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/replacearr/replacearr/internal/domain"
	"golang.org/x/net/proxy"
)

// ProxySource returns an optional proxy URL for outbound HTTP (e.g. plugin layer).
type ProxySource interface {
	ProxyURL() string
}

// Options configures the outbound client stack.
type Options struct {
	AppProxy   domain.ProxyConfig
	PluginURLs []string
	Timeout    time.Duration
}

// Stack builds proxied HTTP clients from AppConfig and/or network_proxy plugins.
type Stack struct {
	Opts Options
}

// Client returns an *http.Client using the stack options.
func (s *Stack) Client() *http.Client {
	if s == nil {
		return New(Options{})
	}
	return New(s.Opts)
}

// New builds an *http.Client that honors AppConfig.Proxy and plugin proxy URLs.
// The first non-empty proxy wins: enabled AppProxy, then the first plugin URL.
func New(opts Options) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	proxyURL := resolveProxyURL(opts)
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err == nil {
			switch strings.ToLower(u.Scheme) {
			case "socks5", "socks5h":
				dialer, err := proxy.FromURL(u, proxy.Direct)
				if err == nil {
					if cd, ok := dialer.(proxy.ContextDialer); ok {
						transport.DialContext = cd.DialContext
					} else {
						transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
							return dialer.Dial(network, addr)
						}
					}
					transport.Proxy = nil
				}
			default:
				transport.Proxy = http.ProxyURL(u)
			}
		}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func resolveProxyURL(opts Options) string {
	if opts.AppProxy.Enabled && opts.AppProxy.Hostname != "" {
		scheme := strings.ToLower(opts.AppProxy.Type)
		if scheme == "" {
			scheme = "http"
		}
		userinfo := ""
		if opts.AppProxy.Username != "" {
			if opts.AppProxy.Password != "" {
				userinfo = url.UserPassword(opts.AppProxy.Username, opts.AppProxy.Password).String() + "@"
			} else {
				userinfo = url.User(opts.AppProxy.Username).String() + "@"
			}
		}
		port := opts.AppProxy.Port
		if port <= 0 {
			port = 8080
		}
		return fmt.Sprintf("%s://%s%s:%d", scheme, userinfo, opts.AppProxy.Hostname, port)
	}
	for _, u := range opts.PluginURLs {
		if strings.TrimSpace(u) != "" {
			return strings.TrimSpace(u)
		}
	}
	return ""
}
