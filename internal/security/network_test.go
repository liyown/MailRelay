package security

import (
	"context"
	"net"
	"net/url"
	"testing"
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
