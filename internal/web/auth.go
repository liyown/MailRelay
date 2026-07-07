package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	sessionCookie = "mailrelay_session"
	csrfHeader    = "X-CSRF-Token"
)

type SessionOptions struct {
	Secret       []byte
	PasswordHash string
	TTL          time.Duration
	Now          func() time.Time
	Random       io.Reader
	SecureCookie bool
}

type SessionManager struct {
	secret       []byte
	passwordHash string
	ttl          time.Duration
	now          func() time.Time
	random       io.Reader
	secure       bool
}

type Session struct {
	Subject string `json:"sub"`
	Expires int64  `json:"exp"`
	CSRF    string `json:"csrf"`
}

func NewSessionManager(options SessionOptions) (*SessionManager, error) {
	if len(options.Secret) < 32 {
		return nil, fmt.Errorf("session secret must contain at least 32 bytes")
	}
	if _, err := parsePasswordHash(options.PasswordHash); err != nil {
		return nil, err
	}
	if options.TTL <= 0 {
		options.TTL = 8 * time.Hour
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Random == nil {
		options.Random = rand.Reader
	}
	return &SessionManager{secret: append([]byte(nil), options.Secret...), passwordHash: options.PasswordHash, ttl: options.TTL, now: options.Now, random: options.Random, secure: options.SecureCookie}, nil
}

func (m *SessionManager) VerifyPassword(password string) bool {
	p, err := parsePasswordHash(m.passwordHash)
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), p.salt, p.time, p.memory, p.threads, uint32(len(p.hash)))
	return subtle.ConstantTimeCompare(actual, p.hash) == 1
}

func (m *SessionManager) Issue(subject string) (*http.Cookie, string, error) {
	csrfBytes := make([]byte, 32)
	if _, err := io.ReadFull(m.random, csrfBytes); err != nil {
		return nil, "", fmt.Errorf("generate csrf token: %w", err)
	}
	csrf := base64.RawURLEncoding.EncodeToString(csrfBytes)
	session := Session{Subject: subject, Expires: m.now().Add(m.ttl).Unix(), CSRF: csrf}
	payload, err := json.Marshal(session)
	if err != nil {
		return nil, "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	value := encoded + "." + m.sign(encoded)
	return &http.Cookie{Name: sessionCookie, Value: value, Path: "/", HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteStrictMode, MaxAge: int(m.ttl.Seconds()), Expires: m.now().Add(m.ttl)}, csrf, nil
}

func (m *SessionManager) Authenticate(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return Session{}, false
	}
	encoded, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(m.sign(encoded))) {
		return Session{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Session{}, false
	}
	var session Session
	if json.Unmarshal(payload, &session) != nil || session.Subject == "" || session.CSRF == "" || session.Expires <= m.now().Unix() {
		return Session{}, false
	}
	return session, true
}

func (m *SessionManager) ValidCSRF(r *http.Request) bool {
	session, ok := m.Authenticate(r)
	if !ok {
		return false
	}
	supplied := r.Header.Get(csrfHeader)
	return supplied != "" && subtle.ConstantTimeCompare([]byte(supplied), []byte(session.CSRF)) == 1
}

func (m *SessionManager) LogoutCookie() *http.Cookie {
	return &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)}
}

func (m *SessionManager) sign(payload string) string {
	h := hmac.New(sha256.New, m.secret)
	_, _ = h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

type passwordParameters struct {
	memory  uint32
	time    uint32
	threads uint8
	salt    []byte
	hash    []byte
}

func HashPassword(password string, random io.Reader) (string, error) {
	if random == nil {
		random = rand.Reader
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(random, salt); err != nil {
		return "", err
	}
	const memory, iterations, threads, keyLen = uint32(64 * 1024), uint32(3), uint8(2), uint32(32)
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memory, iterations, threads, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func parsePasswordHash(encoded string) (passwordParameters, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return passwordParameters{}, fmt.Errorf("invalid Argon2id password hash")
	}
	var p passwordParameters
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return p, fmt.Errorf("invalid Argon2id parameters")
	}
	read := func(value, prefix string, bits int) (uint64, error) {
		if !strings.HasPrefix(value, prefix) {
			return 0, fmt.Errorf("invalid Argon2id parameters")
		}
		return strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, bits)
	}
	memory, err := read(params[0], "m=", 32)
	if err != nil {
		return p, err
	}
	iterations, err := read(params[1], "t=", 32)
	if err != nil {
		return p, err
	}
	threads, err := read(params[2], "p=", 8)
	if err != nil || memory == 0 || memory > 256*1024 || iterations == 0 || iterations > 10 || threads == 0 || threads > 16 {
		return p, fmt.Errorf("invalid Argon2id parameters")
	}
	p.salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(p.salt) < 8 {
		return p, fmt.Errorf("invalid Argon2id salt")
	}
	p.hash, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(p.hash) < 16 {
		return p, fmt.Errorf("invalid Argon2id hash")
	}
	p.memory, p.time, p.threads = uint32(memory), uint32(iterations), uint8(threads)
	return p, nil
}
