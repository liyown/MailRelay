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
	CommandTimeout     string        `yaml:"command_timeout"`
	ConfigReload       bool          `yaml:"config_reload"`
	CatalogNotify      []string      `yaml:"catalog_notify"`
	EnableExperimental bool          `yaml:"enable_experimental"`
	ReplyMaxAttempts   int           `yaml:"reply_max_attempts"`
	QueueMaxAttempts   int           `yaml:"queue_max_attempts"`
	InitialBackoff     time.Duration `yaml:"-"`
	MaxBackoff         time.Duration `yaml:"-"`
}

func (r *Runtime) UnmarshalYAML(n *yaml.Node) error {
	type raw struct {
		CommandTimeout     string   `yaml:"command_timeout"`
		ConfigReload       *bool    `yaml:"config_reload"`
		CatalogNotify      []string `yaml:"catalog_notify"`
		EnableExperimental bool     `yaml:"enable_experimental"`
		ReplyMaxAttempts   int      `yaml:"reply_max_attempts"`
		QueueMaxAttempts   int      `yaml:"queue_max_attempts"`
		InitialBackoff     string   `yaml:"initial_backoff"`
		MaxBackoff         string   `yaml:"max_backoff"`
	}
	var x raw
	if err := n.Decode(&x); err != nil {
		return err
	}
	r.CommandTimeout = x.CommandTimeout
	if x.ConfigReload != nil {
		r.ConfigReload = *x.ConfigReload
	} else {
		r.ConfigReload = true
	}
	r.CatalogNotify = x.CatalogNotify
	r.EnableExperimental = x.EnableExperimental
	r.ReplyMaxAttempts = x.ReplyMaxAttempts
	r.QueueMaxAttempts = x.QueueMaxAttempts
	if x.InitialBackoff != "" {
		d, err := time.ParseDuration(x.InitialBackoff)
		if err != nil {
			return err
		}
		r.InitialBackoff = d
	}
	if x.MaxBackoff != "" {
		d, err := time.ParseDuration(x.MaxBackoff)
		if err != nil {
			return err
		}
		r.MaxBackoff = d
	}
	return nil
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
	c := Config{
		Runtime: Runtime{
			ConfigReload: true,
		},
	}
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
	if c.Runtime.ReplyMaxAttempts == 0 {
		c.Runtime.ReplyMaxAttempts = 5
	}
	if c.Runtime.QueueMaxAttempts == 0 {
		c.Runtime.QueueMaxAttempts = 3
	}
	if c.Runtime.InitialBackoff == 0 {
		c.Runtime.InitialBackoff = time.Minute
	}
	if c.Runtime.MaxBackoff == 0 {
		c.Runtime.MaxBackoff = 30 * time.Minute
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
		if command.HandlerMaturity(cmd.Handler) == "Experimental" && !c.Runtime.EnableExperimental {
			return fmt.Errorf("experimental handler %q requires runtime.enable_experimental", cmd.Handler)
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
	if err := validateCommandGraph(c.Commands); err != nil {
		return err
	}
	byName := make(map[string]command.Command, len(c.Commands))
	for _, cmd := range c.Commands {
		byName[cmd.Name] = cmd
	}
	for _, cmd := range c.Commands {
		if cmd.Handler != "queue" {
			continue
		}
		target, _ := cmd.Config["command"].(string)
		if err := validateQueueSchema(cmd, byName[target]); err != nil {
			return err
		}
	}
	return nil
}

func parameterType(p command.Parameter) string {
	if p.Type == "" {
		return "string"
	}
	return p.Type
}

func validateQueueSchema(wrapper, target command.Command) error {
	for name, source := range wrapper.Parameters {
		destination, ok := target.Parameters[name]
		if !ok {
			return fmt.Errorf("queue command %s parameter %s is not declared by target %s", wrapper.Name, name, target.Name)
		}
		if source.Sensitive || destination.Sensitive {
			return fmt.Errorf("queue command %s cannot persist sensitive parameter %s", wrapper.Name, name)
		}
		if parameterType(source) != parameterType(destination) {
			return fmt.Errorf("queue command %s parameter %s type does not match target %s", wrapper.Name, name, target.Name)
		}
	}
	for name, destination := range target.Parameters {
		if !destination.Required {
			continue
		}
		source, ok := wrapper.Parameters[name]
		if !ok || !source.Required {
			return fmt.Errorf("queue command %s must require target parameter %s", wrapper.Name, name)
		}
	}
	return nil
}

func commandTargets(c command.Command) ([]string, error) {
	switch c.Handler {
	case "workflow":
		raw, ok := c.Config["steps"].([]any)
		if !ok || len(raw) == 0 {
			return nil, fmt.Errorf("workflow %s has no steps", c.Name)
		}
		targets := make([]string, 0, len(raw))
		for i, value := range raw {
			step, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("workflow %s step %d is invalid", c.Name, i+1)
			}
			target, _ := step["command"].(string)
			if target == "" {
				return nil, fmt.Errorf("workflow %s step %d has no command", c.Name, i+1)
			}
			targets = append(targets, target)
		}
		return targets, nil
	case "queue":
		target, _ := c.Config["command"].(string)
		if target == "" {
			return nil, fmt.Errorf("queue %s has no target", c.Name)
		}
		return []string{target}, nil
	default:
		return nil, nil
	}
}

func validateCommandGraph(commands []command.Command) error {
	byName := make(map[string]command.Command, len(commands))
	for _, c := range commands {
		byName[c.Name] = c
	}
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(name string) error {
		if state[name] == 1 {
			return fmt.Errorf("command cycle involving %s", name)
		}
		if state[name] == 2 {
			return nil
		}
		state[name] = 1
		targets, err := commandTargets(byName[name])
		if err != nil {
			return err
		}
		for _, target := range targets {
			if _, ok := byName[target]; !ok {
				return fmt.Errorf("%s target %s is not declared", byName[name].Handler, target)
			}
			if err := visit(target); err != nil {
				return err
			}
		}
		state[name] = 2
		return nil
	}
	for name := range byName {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}
