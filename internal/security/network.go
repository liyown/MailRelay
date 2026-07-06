package security

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type NetworkPolicy struct {
	Hosts    []string
	LookupIP func(context.Context, string) ([]net.IP, error)
}

func (p NetworkPolicy) HTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		u := &url.URL{Scheme: "https", Host: net.JoinHostPort(host, port)}
		if err = p.Check(ctx, u); err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if forbiddenIP(ip) {
				continue
			}
			conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if e == nil {
				return conn, nil
			}
			err = e
		}
		return nil, err
	}
	c := &http.Client{Transport: tr, Timeout: timeout}
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 10 {
			return fmt.Errorf("too many redirects")
		}
		if len(via) > 0 && !strings.EqualFold(req.URL.Hostname(), via[0].URL.Hostname()) {
			return fmt.Errorf("cross-host redirect denied")
		}
		return p.Check(req.Context(), req.URL)
	}
	return c
}

func (p NetworkPolicy) Check(ctx context.Context, u *url.URL) error {
	if u == nil || u.Scheme != "https" {
		return fmt.Errorf("HTTPS is required")
	}
	host := strings.ToLower(u.Hostname())
	ok := false
	for _, h := range p.Hosts {
		if strings.EqualFold(host, h) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("host %s is not allowed", host)
	}
	lookup := p.LookupIP
	if lookup == nil {
		r := net.DefaultResolver
		lookup = func(ctx context.Context, host string) ([]net.IP, error) {
			return r.LookupIP(ctx, "ip", host)
		}
	}
	ips, err := lookup(ctx, host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("host has no addresses")
	}
	for _, ip := range ips {
		if forbiddenIP(ip) {
			return fmt.Errorf("address %s is forbidden", ip)
		}
	}
	return nil
}
func forbiddenIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if v := ip.To4(); v != nil {
		if v[0] == 0 || v[0] >= 224 {
			return true
		}
		if v[0] == 169 && v[1] == 254 {
			return true
		}
		if v[0] == 100 && v[1] >= 64 && v[1] <= 127 {
			return true
		}
	}
	return false
}
