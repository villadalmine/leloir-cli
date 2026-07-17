package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CLI configuration — contexts, kubeconfig-style (spec-m9-cli.md).
// File is written 0600 because it holds API keys, like a kubeconfig.

type cliConfig struct {
	CurrentContext string       `yaml:"current-context"`
	Contexts       []cliContext `yaml:"contexts"`
}

type cliContext struct {
	Name   string `yaml:"name"`
	Server string `yaml:"server"`
	APIKey string `yaml:"apiKey,omitempty"`
	// Tenant applies only in single-user dev mode (?tenant=). With an APIKey
	// the server derives the tenant from the key — this field is ignored.
	Tenant string `yaml:"tenant,omitempty"`
}

func configPath() string {
	if p := os.Getenv("LELOIR_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".leloir-config.yaml"
	}
	return filepath.Join(home, ".config", "leloir", "config.yaml")
}

func loadConfig() (*cliConfig, error) {
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return &cliConfig{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg cliConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath(), err)
	}
	return &cfg, nil
}

func saveConfig(cfg *cliConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *cliConfig) find(name string) *cliContext {
	for i := range c.Contexts {
		if c.Contexts[i].Name == name {
			return &c.Contexts[i]
		}
	}
	return nil
}

// resolveContext applies the precedence: flags > env > file > defaults.
func resolveContext(g globals) (cliContext, error) {
	cfg, err := loadConfig()
	if err != nil {
		return cliContext{}, err
	}

	name := g.context
	if name == "" {
		name = os.Getenv("LELOIR_CONTEXT")
	}
	if name == "" {
		name = cfg.CurrentContext
	}

	ctx := cliContext{Server: "http://localhost:8081"}
	if name != "" {
		found := cfg.find(name)
		if found == nil {
			return cliContext{}, fmt.Errorf("context %q no existe (leloir config view)", name)
		}
		ctx = *found
	}

	if v := os.Getenv("LELOIR_SERVER"); v != "" {
		ctx.Server = v
	}
	if v := os.Getenv("LELOIR_API_KEY"); v != "" {
		ctx.APIKey = v
	}
	if g.server != "" {
		ctx.Server = g.server
	}
	if g.apiKey != "" {
		ctx.APIKey = g.apiKey
	}
	return ctx, nil
}

// maskKey renders lk_abcdef...1234 for display — the full key never leaves
// the config file once created.
func maskKey(k string) string {
	if k == "" {
		return "(none — single-user)"
	}
	if len(k) < 12 {
		return "lk_…"
	}
	return k[:8] + "…" + k[len(k)-4:]
}

func cmdConfig(g globals, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: leloir config view | use-context <name> | set-context <name> --server URL [--api-key K] [--tenant T]")
		return 2
	}
	switch args[0] {
	case "view":
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("archivo: %s\n", configPath())
		fmt.Printf("current-context: %s\n", cfg.CurrentContext)
		for _, c := range cfg.Contexts {
			marker := " "
			if c.Name == cfg.CurrentContext {
				marker = "*"
			}
			fmt.Printf("%s %-14s %-28s key=%s", marker, c.Name, c.Server, maskKey(c.APIKey))
			if c.Tenant != "" {
				fmt.Printf(" tenant=%s(dev)", c.Tenant)
			}
			fmt.Println()
		}
		return 0

	case "use-context":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "uso: leloir config use-context <name>")
			return 2
		}
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		if cfg.find(args[1]) == nil {
			fmt.Fprintf(os.Stderr, "error: context %q no existe\n", args[1])
			return 1
		}
		cfg.CurrentContext = args[1]
		if err := saveConfig(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("context activo: %s\n", args[1])
		return 0

	case "set-context":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "uso: leloir config set-context <name> --server URL [--api-key K] [--tenant T]")
			return 2
		}
		name := args[1]
		fs := newFlagSet("config set-context")
		server := fs.String("server", "", "URL del control plane")
		apiKey := fs.String("api-key", "", "API key lk_...")
		tenant := fs.String("tenant", "", "tenant (solo modo single-user dev)")
		if err := fs.Parse(args[2:]); err != nil {
			return 2
		}
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		ctx := cfg.find(name)
		if ctx == nil {
			cfg.Contexts = append(cfg.Contexts, cliContext{Name: name})
			ctx = &cfg.Contexts[len(cfg.Contexts)-1]
		}
		if *server != "" {
			ctx.Server = *server
		}
		if ctx.Server == "" {
			ctx.Server = "http://localhost:8081"
		}
		if *apiKey != "" {
			ctx.APIKey = *apiKey
		}
		if *tenant != "" {
			ctx.Tenant = *tenant
		}
		if cfg.CurrentContext == "" {
			cfg.CurrentContext = name
		}
		if err := saveConfig(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("context %q guardado (%s, key=%s)\n", name, ctx.Server, maskKey(ctx.APIKey))
		return 0

	default:
		fmt.Fprintf(os.Stderr, "error: subcomando desconocido %q\n", args[0])
		return 2
	}
}
