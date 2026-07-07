package mailbox

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/liyown/MailRelay/internal/config"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIMAPReceiverFetchAndMarkSeenAgainstLocalTLSServer(t *testing.T) {
	serverTLS, clientTLS := testTLS(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	raw := "From: me@example.com\r\nSubject: help\r\nMessage-ID: <imap-1>\r\n\r\n"
	var mu sync.Mutex
	stored := false
	handle := func() {
		c, e := ln.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		rw := bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c))
		write := func(s string) { fmt.Fprint(rw, s); rw.Flush() }
		write("* OK local IMAP ready\r\n")
		for {
			line, e := rw.ReadString('\n')
			if e != nil {
				return
			}
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			tag := parts[0]
			upper := strings.ToUpper(line)
			switch {
			case strings.Contains(upper, " LOGIN "):
				write(tag + " OK LOGIN completed\r\n")
			case strings.Contains(upper, " SELECT "):
				write("* 1 EXISTS\r\n* FLAGS (\\Seen)\r\n" + tag + " OK [READ-WRITE] SELECT completed\r\n")
			case strings.Contains(upper, "UID SEARCH"):
				write("* SEARCH 7\r\n" + tag + " OK SEARCH completed\r\n")
			case strings.Contains(upper, "UID FETCH"):
				write(fmt.Sprintf("* 1 FETCH (UID 7 BODY[] {%d}\r\n%s)\r\n%s OK FETCH completed\r\n", len(raw), raw, tag))
			case strings.Contains(upper, "UID STORE"):
				mu.Lock()
				stored = true
				mu.Unlock()
				write(tag + " OK STORE completed\r\n")
			case strings.Contains(upper, "LOGOUT"):
				write("* BYE logout\r\n" + tag + " OK LOGOUT completed\r\n")
				return
			default:
				write(tag + " BAD unsupported\r\n")
			}
		}
	}
	go handle()
	r := NewIMAP(config.MailEndpoint{Address: ln.Addr().String(), Username: "u", Password: "p", Mailbox: "INBOX"})
	r.tlsConfig = clientTLS
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msgs, err := r.Fetch(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].UID != 7 || string(msgs[0].Body) != raw {
		t.Fatalf("%#v", msgs)
	}
	go handle()
	if err = r.MarkSeen(ctx, 7); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	ok := stored
	mu.Unlock()
	if !ok {
		t.Fatal("UID STORE was not received")
	}
}
