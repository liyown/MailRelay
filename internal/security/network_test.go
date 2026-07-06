package security

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestNetworkPolicy(t *testing.T) {
	p := NetworkPolicy{Hosts: []string{"api.example.com"}, LookupIP: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("127.0.0.1")}, nil }}
	u, _ := url.Parse("https://api.example.com/x")
	if err := p.Check(context.Background(), u); err == nil {
		t.Fatal("expected private IP denial")
	}
	p.LookupIP = func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }
	if err := p.Check(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	u, _ = url.Parse("http://api.example.com")
	if err := p.Check(context.Background(), u); err == nil {
		t.Fatal("expected https denial")
	}
}

func TestProtectedClientRejectsCrossHostRedirect(t *testing.T) {
	p := NetworkPolicy{Hosts: []string{"api.example.com", "other.example.com"}}
	c := p.HTTPClient(time.Second)
	first, _ := http.NewRequest("GET", "https://api.example.com/start", nil)
	next, _ := http.NewRequest("GET", "https://other.example.com/end", nil)
	if err := c.CheckRedirect(next, []*http.Request{first}); err == nil {
		t.Fatal("expected cross-host redirect denial")
	}
}
