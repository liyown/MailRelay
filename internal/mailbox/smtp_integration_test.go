package mailbox

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/becomeopc/opc-mailrelay/internal/config"
	"strings"
	"testing"
	"time"
)

func TestSMTPSenderAgainstLocalTLSServer(t *testing.T) {
	serverTLS, clientTLS := testTLS(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	received := make(chan string, 1)
	go func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		rw := bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c))
		write := func(s string) { fmt.Fprint(rw, s); rw.Flush() }
		write("220 local ESMTP\r\n")
		var data strings.Builder
		inData := false
		for {
			line, e := rw.ReadString('\n')
			if e != nil {
				return
			}
			trim := strings.TrimRight(line, "\r\n")
			if inData {
				if trim == "." {
					inData = false
					received <- data.String()
					write("250 queued\r\n")
				} else {
					data.WriteString(line)
				}
				continue
			}
			upper := strings.ToUpper(trim)
			switch {
			case strings.HasPrefix(upper, "EHLO"):
				write("250-local\r\n250 OK\r\n")
			case strings.HasPrefix(upper, "MAIL FROM"):
				write("250 OK\r\n")
			case strings.HasPrefix(upper, "RCPT TO"):
				write("250 OK\r\n")
			case upper == "DATA":
				inData = true
				write("354 end with dot\r\n")
			case upper == "QUIT":
				write("221 bye\r\n")
				return
			default:
				write("250 OK\r\n")
			}
		}
	}()
	s := NewSMTP(config.MailEndpoint{Address: ln.Addr().String(), From: "relay@example.com"})
	s.tlsConfig = clientTLS
	msg := []byte("From: relay@example.com\r\nTo: me@example.com\r\n\r\nhello\r\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = s.Send(ctx, "me@example.com", msg); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		if !strings.Contains(got, "hello") {
			t.Fatal(got)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
