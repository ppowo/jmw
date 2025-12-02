package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration
type Config struct {
	Projects     map[string]*ProjectConfig `yaml:"projects"`
	RestartRules *RestartRules             `yaml:"restart_rules"`
}

// ProjectConfig represents configuration for a single project
type ProjectConfig struct {
	// Path detection
	BasePath string `yaml:"base_path"` // Base path for detection and multi-module root when it contains a parent POM

	// Build settings
	DefaultProfile    string              `yaml:"default_profile"`    // Default Maven profile (sinfomar only)
	AvailableProfiles []string            `yaml:"available_profiles"` // Valid profiles
	ProfileOverrides  map[string][]string `yaml:"profile_overrides"`  // Profile override rules
	SkipTests         bool                `yaml:"skip_tests"`         // Skip tests flag

	// WildFly settings
	WildFlyRoot string `yaml:"wildfly_root"` // WildFly installation root
	WildFlyMode string `yaml:"wildfly_mode"` // "domain" or "standalone"
	ServerGroup string `yaml:"server_group"` // For domain mode

	// Remote deployment (for instructions only)
	Remote *RemoteConfig `yaml:"remote"`

	// Module mappings
	// Key: module name, Value: path for global modules or empty string for normal deployment
	Modules map[string]string `yaml:"modules"`

	// Deprecated: kept for backwards compatibility
	GlobalModules  map[string]*GlobalModuleGroup `yaml:"global_modules,omitempty"`
	IgnoredModules []string                      `yaml:"ignored_modules,omitempty"`
}

// RemoteConfig represents remote server configuration
type RemoteConfig struct {
	Host        string `yaml:"host"`
	User        string `yaml:"user"`
	WildFlyPath string `yaml:"wildfly_path"`
	RestartCmd  string `yaml:"restart_cmd"`
}

// GlobalModuleGroup represents a group of global modules
type GlobalModuleGroup struct {
	Modules    []string `yaml:"modules"`
	LocalPath  string   `yaml:"local_path"`
	RemotePath string   `yaml:"remote_path"`
	Path       string   `yaml:"path"` // Simplified: same path for local and remote
}

// RestartRules defines when restarts are needed
type RestartRules struct {
	GlobalModule bool              `yaml:"global_module"`
	Patterns     []*RestartPattern `yaml:"patterns"`
}

// RestartPattern defines a pattern that requires restart
type RestartPattern struct {
	Match    string `yaml:"match"`
	Reason   string `yaml:"reason"`
	Severity string `yaml:"severity"` // "required" or "recommended"
}

// Load reads and parses the configuration file
func Load(configPath string) (*Config, error) {
	// Resolve config path
	if !filepath.IsAbs(configPath) {
		// Look in gmw directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, "Personal", "gmw", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand tildes in paths
	if err := cfg.expandTildes(); err != nil {
		return nil, fmt.Errorf("failed to expand paths: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Projects) == 0 {
		return fmt.Errorf("no projects defined in configuration")
	}

	for name, proj := range c.Projects {
		if proj.BasePath == "" {
			return fmt.Errorf("project %s: base_path is required", name)
		}
		if proj.WildFlyRoot == "" {
			return fmt.Errorf("project %s: wildfly_root is required", name)
		}
		if proj.WildFlyMode != "domain" && proj.WildFlyMode != "standalone" {
			return fmt.Errorf("project %s: wildfly_mode must be 'domain' or 'standalone'", name)
		}
	}

	return nil
}

// GetModuleDeploymentPath returns the deployment path for a module
// Returns (path, isGlobal, exists)
func (p *ProjectConfig) GetModuleDeploymentPath(moduleName string) (string, bool, bool) {
	path, exists := p.Modules[moduleName]
	if !exists {
		return "", false, false
	}

	// Empty string means normal deployment
	if path == "" {
		return "", false, true
	}

	// Non-empty path means global module
	return path, true, true
}

// GetGlobalModulePath returns the local path for a global module group
func (g *GlobalModuleGroup) GetLocalPath() string {
	if g.Path != "" {
		return g.Path
	}
	return g.LocalPath
}

// GetRemotePath returns the remote path for a global module group
func (g *GlobalModuleGroup) GetRemotePath() string {
	if g.Path != "" {
		return g.Path
	}
	return g.RemotePath
}

// IsIgnored checks if a module should be ignored
func (p *ProjectConfig) IsIgnored(moduleName string) bool {
	return slices.Contains(p.IgnoredModules, moduleName)
}

// GetProfileArgs returns Maven profile arguments for a given profile
func (p *ProjectConfig) GetProfileArgs(profile string) []string {
	if profile == "" {
		// Check for overrides for empty profile first
		if overrides, ok := p.ProfileOverrides[""]; ok {
			var args []string
			for _, prof := range overrides {
				args = append(args, "-P"+prof)
			}
			return args
		}
		// Fall back to default profile
		if p.DefaultProfile != "" {
			return []string{"-P" + p.DefaultProfile}
		}
		return []string{}
	}

	// Check for overrides
	if overrides, ok := p.ProfileOverrides[profile]; ok {
		var args []string
		for _, prof := range overrides {
			args = append(args, "-P"+prof)
		}
		return args
	}

	// Default: just use the profile
	return []string{"-P" + profile}
}

// expandTildes expands ~ in all path fields to the user's home directory
func (c *Config) expandTildes() error {
	for _, proj := range c.Projects {
		// Expand paths
		var err error
		proj.BasePath, err = homedir.Expand(proj.BasePath)
		if err != nil {
			return fmt.Errorf("failed to expand base_path: %w", err)
		}
		proj.WildFlyRoot, err = homedir.Expand(proj.WildFlyRoot)
		if err != nil {
			return fmt.Errorf("failed to expand wildfly_root: %w", err)
		}

		// Expand remote paths
		if proj.Remote != nil {
			proj.Remote.WildFlyPath, err = homedir.Expand(proj.Remote.WildFlyPath)
			if err != nil {
				return fmt.Errorf("failed to expand remote wildfly_path: %w", err)
			}
		}

		// Expand global module paths
		for key, gm := range proj.GlobalModules {
			if gm.LocalPath != "" {
				gm.LocalPath, err = homedir.Expand(gm.LocalPath)
				if err != nil {
					return fmt.Errorf("failed to expand global module path: %w", err)
				}
			}
			if gm.RemotePath != "" {
				gm.RemotePath, err = homedir.Expand(gm.RemotePath)
				if err != nil {
					return fmt.Errorf("failed to expand global module remote path: %w", err)
				}
			}
			if gm.Path != "" {
				gm.Path, err = homedir.Expand(gm.Path)
				if err != nil {
					return fmt.Errorf("failed to expand global module path: %w", err)
				}
			}
			proj.GlobalModules[key] = gm
		}

		// Expand module paths in modules map
		for key, path := range proj.Modules {
			proj.Modules[key], err = homedir.Expand(path)
			if err != nil {
				return fmt.Errorf("failed to expand module path: %w", err)
			}
		}
	}

	return nil
}
