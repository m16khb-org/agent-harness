package webfetch

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

func sanitizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	if u.User != nil {
		u.User = url.User("<redacted>")
	}
	return u.String()
}

func validateFetchURL(ctx context.Context, raw string, allowPrivate bool, resolver Resolver) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("URL userinfo is not allowed")
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("URL host is required")
	}
	if isUnsafeHost(host, allowPrivate) {
		return nil, fmt.Errorf("unsafe host %q", host)
	}
	if net.ParseIP(strings.Trim(host, "[]")) == nil {
		if resolver == nil {
			resolver = net.DefaultResolver
		}
		addrs, err := resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("host resolution failed for %q: %w", host, err)
		}
		for _, addr := range addrs {
			if isUnsafeHost(addr.IP.String(), allowPrivate) {
				return nil, fmt.Errorf("unsafe resolved host %q -> %s", host, addr.IP.String())
			}
		}
	}
	return u, nil
}

func isUnsafeHost(host string, allowPrivate bool) bool {
	lower := strings.ToLower(strings.Trim(host, "[]"))
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return !allowPrivate
	}
	ip := net.ParseIP(lower)
	if ip == nil {
		return false
	}
	if isMetadataIP(ip) {
		return true
	}
	if isReservedIP(ip) {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return !allowPrivate
	}
	return false
}

func isMetadataIP(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	return v4[0] == 169 && v4[1] == 254 && v4[2] == 169 && v4[3] == 254
}

func isReservedIP(ip net.IP) bool {
	v4 := ip.To4()
	if v4 != nil {
		switch {
		case v4[0] == 0:
			return true
		case v4[0] == 100 && v4[1]&0b11000000 == 64:
			return true
		case v4[0] == 192 && v4[1] == 0 && v4[2] == 0:
			return true
		case v4[0] == 192 && v4[1] == 0 && v4[2] == 2:
			return true
		case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19):
			return true
		case v4[0] == 198 && v4[1] == 51 && v4[2] == 100:
			return true
		case v4[0] == 203 && v4[1] == 0 && v4[2] == 113:
			return true
		case v4[0] >= 240:
			return true
		}
		return false
	}
	return strings.HasPrefix(strings.ToLower(ip.String()), "2001:db8:")
}
