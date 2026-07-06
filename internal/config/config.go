package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	"gopkg.in/yaml.v3"
)

type MailEndpoint struct {
	Address, Username, Password, From, Mailbox string
	PollInterval                               time.Duration
}

func (e *MailEndpoint) UnmarshalYAML(n *yaml.Node) error {
	type raw struct {
		Address, Username, Password, From, Mailbox string
		PollInterval                               string `yaml:"poll_interval"`
	}
	var r raw
	if err := n.Decode(&r); err != nil {
		return err
	}
	e.Address, e.Username, e.Password, e.From, e.Mailbox = r.Address, r.Username, r.Password, r.From, r.Mailbox
	if r.PollInterval != "" {
		d, err := time.ParseDuration(r.PollInterval)
		if err != nil {
			return err
		}
		e.PollInterval = d
	}
	return nil
}

type Mail struct {
	IMAP MailEndpoint `yaml:"imap"`
	SMTP MailEndpoint `yaml:"smtp"`
}
type Security struct {
	Token           string   `yaml:"token"`
	Allow           []string `yaml:"allow"`
	HTTPHosts       []string `yaml:"http_hosts"`
	ExecutableRoots []string `yaml:"executable_roots"`
}
type Storage struct {
	Path string `yaml:"path"`
}
type Runtime struct {
	CommandTimeout string   `yaml:"command_timeout"`
	ConfigReload   bool     `yaml:"config_reload"`
	CatalogNotify  []string `yaml:"catalog_notify"`
}
type Config struct {
	Mail     Mail                      `yaml:"mail"`
	Security Security                  `yaml:"security"`
	Storage  Storage                   `yaml:"storage"`
	Runtime  Runtime                   `yaml:"runtime"`
	Handlers map[string]map[string]any `yaml:"handlers"`
	Commands []command.Command         `yaml:"commands"`
}

var envRE = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	missing := ""
	b = envRE.ReplaceAllFunc(b, func(s []byte) []byte {
		name := envRE.FindSubmatch(s)[1]
		v, ok := os.LookupEnv(string(name))
		if !ok {
			missing = string(name)
		}
		return []byte(v)
	})
	if missing != "" {
		return nil, fmt.Errorf("environment variable %s is not set", missing)
	}
	var c Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err = dec.Decode(&c); err != nil {
		return nil, err
	}
	if c.Storage.Path == "" {
		c.Storage.Path = "data/mailrelay.db"
	}
	if !filepath.IsAbs(c.Storage.Path) {
		c.Storage.Path = filepath.Join(filepath.Dir(path), c.Storage.Path)
	}
	if c.Mail.IMAP.Mailbox == "" {
		c.Mail.IMAP.Mailbox = "INBOX"
	}
	if c.Mail.IMAP.PollInterval == 0 {
		c.Mail.IMAP.PollInterval = 30 * time.Second
	}
	if err = c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Security.Token) == "" {
		return fmt.Errorf("security.token is required")
	}
	if len(c.Security.Allow) == 0 {
		return fmt.Errorf("security.allow is required")
	}
	seen := map[string]bool{}
	validName := regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	hosts := map[string]bool{}
	for _, h := range c.Security.HTTPHosts {
		hosts[strings.ToLower(h)] = true
	}
	known := map[string]bool{"http": true, "webhook": true, "workflow": true, "plugin": true, "shell": true, "agent": true, "mcp": true, "queue": true}
	for _, cmd := range c.Commands {
		if !validName.MatchString(cmd.Name) || cmd.Name == "help" {
			return fmt.Errorf("invalid or reserved command name %q", cmd.Name)
		}
		if seen[cmd.Name] {
			return fmt.Errorf("duplicate command %q", cmd.Name)
		}
		seen[cmd.Name] = true
		if !known[cmd.Handler] {
			return fmt.Errorf("unknown handler %q", cmd.Handler)
		}
		for n, p := range cmd.Parameters {
			if n == "_token" {
				return fmt.Errorf("reserved parameter _token")
			}
			switch p.Type {
			case "", "string", "integer", "number", "boolean":
			default:
				return fmt.Errorf("parameter %s has unsupported type", n)
			}
		}
		if cmd.Handler == "shell" || cmd.Handler == "plugin" {
			x, _ := cmd.Config["executable"].(string)
			if !filepath.IsAbs(x) {
				return fmt.Errorf("%s executable must be absolute", cmd.Name)
			}
		}
		if cmd.Handler == "http" || cmd.Handler == "webhook" {
			u, _ := cmd.Config["url"].(string)
			ok := false
			for h := range hosts {
				if strings.Contains(strings.ToLower(u), "://"+h+"/") || strings.HasSuffix(strings.ToLower(u), "://"+h) {
					ok = true
				}
			}
			if !ok {
				return fmt.Errorf("%s URL host is not allowlisted", cmd.Name)
			}
		}
	}
	return nil
}
