package detector

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ppowo/gmw/internal/config"
)

// ProjectInfo contains detected project information
type ProjectInfo struct {
	Name           string // "sinfomar" or "mto"
	Config         *config.ProjectConfig
	ModuleName     string // Module name (e.g., "EJBPcs", "EJBMto")
	ModulePath     string // Absolute path to module directory
	RepoRoot       string // Repository root (derived from base_path or module_path)
	Packaging      string // "jar" or "war"
	DeploymentPath string // Module deployment path (for global modules)
	IsGlobalModule bool   // Whether this is a global module
}

// PomXML represents a minimal Maven POM structure
type PomXML struct {
	XMLName    xml.Name `xml:"project"`
	ArtifactId string   `xml:"artifactId"`
	Packaging  string   `xml:"packaging"`
}

// hasParentPom checks if a directory has a pom.xml in it
func hasParentPom(path string) bool {
	if path == "" {
		return false
	}
	pomPath := filepath.Join(path, "pom.xml")
	_, err := os.Stat(pomPath)
	return err == nil
}

// GetWorkingDirectory returns the current working directory
func GetWorkingDirectory() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return cwd, nil
}

// DetectProject detects which project we're in based on the current directory
func DetectProject(cwd string, cfg *config.Config) (*ProjectInfo, error) {
	// Try to match against configured projects
	for projectName, projectConfig := range cfg.Projects {
		// Use filepath.Rel to check if cwd is within the base path
		rel, err := filepath.Rel(projectConfig.BasePath, cwd)
		if err != nil {
			continue // Skip if path comparison fails
		}
		// Success if rel doesn't start with ".." (meaning cwd is within base_path)
		if !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
			info := &ProjectInfo{
				Name:   projectName,
				Config: projectConfig,
			}

			// Find the module
			if err := info.detectModule(cwd); err != nil {
				return nil, fmt.Errorf("failed to detect module: %w", err)
			}

			return info, nil
		}
	}

	return nil, fmt.Errorf("current directory is not part of any configured project")
}

// detectModule finds the Maven module information
func (p *ProjectInfo) detectModule(cwd string) error {
	// Find pom.xml by walking up the directory tree
	pomPath, err := findPomXML(cwd)
	if err != nil {
		return fmt.Errorf("failed to find pom.xml: %w", err)
	}

	// Parse pom.xml
	pom, err := parsePomXML(pomPath)
	if err != nil {
		return fmt.Errorf("failed to parse pom.xml: %w", err)
	}

	p.ModulePath = filepath.Dir(pomPath)
	p.ModuleName = filepath.Base(p.ModulePath) // Use folder name instead of artifactId
	p.Packaging = pom.Packaging
	if p.Packaging == "" {
		p.Packaging = "jar" // Default to jar if not specified
	}

	// For multi-module projects, determine the repo root.
	// If base_path contains a parent POM, use it as the repo root (multi-module build).
	// Otherwise, use module path as repo root (single-module build).
	if hasParentPom(p.Config.BasePath) {
		// Base path is a multi-module Maven root - build from there
		p.RepoRoot = p.Config.BasePath
	} else {
		// Single module project - build from module directory
		p.RepoRoot = p.ModulePath
	}

	// Check if module exists in config
	deploymentPath, isGlobal, exists := p.Config.GetModuleDeploymentPath(p.ModuleName)
	if !exists {
		return fmt.Errorf("module '%s' is not configured in config.yaml\n\nPlease add it to the 'modules' section for project '%s':\n  modules:\n    %s: \"\"  # for normal deployment\n    # or\n    %s: \"modules/path/main\"  # for global module",
			p.ModuleName, p.Name, p.ModuleName, p.ModuleName)
	}

	p.DeploymentPath = deploymentPath
	p.IsGlobalModule = isGlobal

	return nil
}

// findPomXML walks up the directory tree to find pom.xml
func findPomXML(startDir string) (string, error) {
	currentDir := startDir

	for {
		pomPath := filepath.Join(currentDir, "pom.xml")
		if _, err := os.Stat(pomPath); err == nil {
			return pomPath, nil
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached root
			break
		}
		currentDir = parentDir
	}

	return "", fmt.Errorf("pom.xml not found in directory tree starting from %s", startDir)
}

// parsePomXML parses a pom.xml file
func parsePomXML(pomPath string) (*PomXML, error) {
	data, err := os.ReadFile(pomPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pom.xml: %w", err)
	}

	var pom PomXML
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	return &pom, nil
}

// GetGlobalModulePath returns the full deployment path for global modules
func (p *ProjectInfo) GetGlobalModulePath() string {
	if p.IsGlobalModule {
		return filepath.Join(p.Config.WildFlyRoot, p.DeploymentPath)
	}
	return ""
}

// GetBuildCommand returns the Maven build command for this module
func (p *ProjectInfo) GetBuildCommand(profile string) []string {
	args := []string{"mvn", "clean"}

	// For mto multi-module projects, build from repo root with -pl flag
	if p.RepoRoot != p.ModulePath {
		// Multi-module build
		relPath, err := filepath.Rel(p.RepoRoot, p.ModulePath)
		if err != nil {
			return args // Fallback to single-module build
		}
		args = append(args, "package", "-pl", relPath, "-am")
	} else {
		// Single module build
		if p.Packaging == "jar" {
			args = append(args, "install")
		} else {
			args = append(args, "package")
		}
	}

	// Add profile arguments
	profileArgs := p.Config.GetProfileArgs(profile)
	args = append(args, profileArgs...)

	// Add skip tests if configured
	if p.Config.SkipTests {
		args = append(args, "-DskipTests")
	}

	return args
}