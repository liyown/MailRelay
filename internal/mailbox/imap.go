package mailbox

import (
	"bytes"
	"context"
	"crypto/tls"
	"github.com/becomeopc/opc-mailrelay/internal/config"
	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"io"
	"sort"
)

type RawMessage struct {
	UID  uint32
	Body []byte
}
type Receiver interface {
	Fetch(context.Context, int) ([]RawMessage, error)
	MarkSeen(context.Context, uint32) error
	Idle(context.Context) error
}
type IMAPReceiver struct{ cfg config.MailEndpoint }

func NewIMAP(c config.MailEndpoint) *IMAPReceiver { return &IMAPReceiver{c} }
func (i *IMAPReceiver) connect() (*client.Client, error) {
	c, err := client.DialTLS(i.cfg.Address, &tls.Config{MinVersion: tls.VersionTLS12})
	if err != nil {
		return nil, err
	}
	if err = c.Login(i.cfg.Username, i.cfg.Password); err != nil {
		c.Logout()
		return nil, err
	}
	if _, err = c.Select(i.cfg.Mailbox, false); err != nil {
		c.Logout()
		return nil, err
	}
	return c, nil
}
func (i *IMAPReceiver) Fetch(ctx context.Context, limit int) ([]RawMessage, error) {
	c, err := i.connect()
	if err != nil {
		return nil, err
	}
	defer c.Logout()
	crit := imap.NewSearchCriteria()
	crit.WithoutFlags = []string{imap.SeenFlag}
	uids, err := c.UidSearch(crit)
	if err != nil {
		return nil, err
	}
	sort.Slice(uids, func(a, b int) bool { return uids[a] < uids[b] })
	if limit > 0 && len(uids) > limit {
		uids = uids[:limit]
	}
	if len(uids) == 0 {
		return nil, nil
	}
	set := new(imap.SeqSet)
	set.AddNum(uids...)
	section := &imap.BodySectionName{}
	ch := make(chan *imap.Message, len(uids))
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(set, []imap.FetchItem{imap.FetchUid, section.FetchItem()}, ch) }()
	var out []RawMessage
	for m := range ch {
		r := m.GetBody(section)
		if r == nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(r, 25<<20))
		if err != nil {
			return nil, err
		}
		out = append(out, RawMessage{UID: m.Uid, Body: b})
	}
	if err = <-done; err != nil {
		return nil, err
	}
	return out, nil
}
func (i *IMAPReceiver) MarkSeen(ctx context.Context, uid uint32) error {
	c, err := i.connect()
	if err != nil {
		return err
	}
	defer c.Logout()
	set := new(imap.SeqSet)
	set.AddNum(uid)
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	return c.UidStore(set, item, []any{imap.SeenFlag}, nil)
}
func (i *IMAPReceiver) Idle(ctx context.Context) error {
	c, err := i.connect()
	if err != nil {
		return err
	}
	defer c.Logout()
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- idle.NewClient(c).Idle(stop) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		close(stop)
		<-done
		return ctx.Err()
	}
}
func RawReader(m RawMessage) io.ReadCloser { return io.NopCloser(bytes.NewReader(m.Body)) }
