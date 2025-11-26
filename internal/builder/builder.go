package builder

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ppowo/gmw/internal/config"
	"github.com/ppowo/gmw/internal/detector"
)

// Builder handles Maven build operations
type Builder struct {
	cfg         *config.Config
	projectInfo *detector.ProjectInfo
}

// NewBuilder creates a new Builder instance
func NewBuilder(cfg *config.Config, projectInfo *detector.ProjectInfo) *Builder {
	return &Builder{
		cfg:         cfg,
		projectInfo: projectInfo,
	}
}

// Build builds the Maven module
func (b *Builder) Build(profile string) error {
	// Validate profile if provided
	if profile != "" && !b.isValidProfile(profile) {
		return fmt.Errorf("invalid profile '%s' for project %s", profile, b.projectInfo.Name)
	}

	// Show build information
	b.showBuildInfo(profile)

	// Get build command
	buildCmd := b.getBuildCommand(profile)

	// Show command
	fmt.Println("\nWill execute:")
	fmt.Printf("  %s\n", strings.Join(buildCmd, " "))

	// Ask for confirmation
	if !b.confirm("Proceed?") {
		fmt.Println("Build cancelled.")
		return nil
	}

	// Execute build
	fmt.Println("\n📦 Building...")
	if err := b.executeBuild(buildCmd); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// For multi-module JARs, also run install
	if b.projectInfo.Config.RepoRoot != "" && b.projectInfo.Packaging == "jar" {
		installCmd := b.getInstallCommand()
		fmt.Println("\n📦 Installing JAR to local repository...")
		if err := b.executeBuild(installCmd); err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
	}

	// Show success
	fmt.Println("\n✅ Build completed successfully!")
	b.showArtifacts()
	b.showRestartGuidance()
	b.showRemoteDeploymentGuide()

	return nil
}

// showBuildInfo displays information about the build
func (b *Builder) showBuildInfo(profile string) {
	fmt.Printf("→ Detected: %s / %s\n", b.projectInfo.Name, b.projectInfo.ModuleName)
	fmt.Printf("→ Packaging: %s\n", b.projectInfo.Packaging)

	if b.projectInfo.Config.RepoRoot != "" {
		fmt.Printf("→ Multi-module build from root\n")
	}

	if profile != "" {
		fmt.Printf("→ Profile: %s\n", profile)
	} else if b.projectInfo.Config.DefaultProfile != "" {
		fmt.Printf("→ Profile: %s (default)\n", b.projectInfo.Config.DefaultProfile)
	}
}

// getBuildCommand constructs the Maven build command
func (b *Builder) getBuildCommand(profile string) []string {
	cmd := []string{"mvn", "clean"}

	// For multi-module projects (mto), build from repo root
	if b.projectInfo.Config.RepoRoot != "" && b.projectInfo.Config.RepoRoot != b.projectInfo.ModulePath {
		relPath, _ := filepath.Rel(b.projectInfo.Config.RepoRoot, b.projectInfo.ModulePath)
		cmd = append(cmd, "package", "-pl", relPath, "-am")
	} else {
		// Single module build
		if b.projectInfo.Packaging == "jar" {
			cmd = append(cmd, "install")
		} else {
			cmd = append(cmd, "package")
		}
	}

	// Add profile arguments
	profileArgs := b.projectInfo.Config.GetProfileArgs(profile)
	cmd = append(cmd, profileArgs...)

	// Add skip tests if configured
	if b.projectInfo.Config.SkipTests {
		cmd = append(cmd, "-DskipTests")
	}

	return cmd
}

// getInstallCommand constructs the install command for multi-module projects
func (b *Builder) getInstallCommand() []string {
	relPath, _ := filepath.Rel(b.projectInfo.Config.RepoRoot, b.projectInfo.ModulePath)
	return []string{"mvn", "install", "-pl", relPath, "-DskipTests"}
}

// executeBuild runs the Maven command
func (b *Builder) executeBuild(cmdArgs []string) error {
	// Determine working directory
	workDir := b.projectInfo.ModulePath
	if b.projectInfo.Config.RepoRoot != "" {
		workDir = b.projectInfo.Config.RepoRoot
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// showArtifacts displays information about built artifacts
func (b *Builder) showArtifacts() {
	targetDir := filepath.Join(b.projectInfo.ModulePath, "target")

	fmt.Println("\n📦 Artifacts built:")

	// Look for JAR files
	if entries, err := os.ReadDir(targetDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".jar") && !strings.Contains(name, "sources") && !strings.Contains(name, "javadoc") {
				fmt.Printf("   📄 JAR: %s\n", name)
			}
			if strings.HasSuffix(name, ".war") {
				fmt.Printf("   📄 WAR: %s\n", name)
			}
		}
	}
}

// showRemoteDeploymentGuide displays instructions for deploying to remote/test environment
func (b *Builder) showRemoteDeploymentGuide() {
	// Only show if remote config exists
	if b.projectInfo.Config.Remote == nil {
		return
	}

	remote := b.projectInfo.Config.Remote
	targetDir := filepath.Join(b.projectInfo.ModulePath, "target")

	// Find the artifact
	var artifactName string
	if entries, err := os.ReadDir(targetDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".war") {
				artifactName = name
				break
			}
			if strings.HasSuffix(name, ".jar") && !strings.Contains(name, "sources") && !strings.Contains(name, "javadoc") {
				artifactName = name
			}
		}
	}

	if artifactName == "" {
		return
	}

	artifactPath := filepath.Join(targetDir, artifactName)
	isGlobal := b.projectInfo.IsGlobalModule

	fmt.Println("\n📝 REMOTE DEPLOYMENT GUIDE (TEST)")
	fmt.Println(strings.Repeat("━", 50))
	fmt.Printf("Host: %s@%s\n\n", remote.User, remote.Host)

	if isGlobal {
		remotePath := filepath.Join(remote.WildFlyPath, b.projectInfo.DeploymentPath)
		fmt.Println("1. Copy artifact:")
		fmt.Printf("   scp %s %s@%s:%s\n", artifactPath, remote.User, remote.Host, remotePath)
		fmt.Println("\n2. Restart WildFly:")
		fmt.Printf("   ssh %s@%s \"%s\"\n", remote.User, remote.Host, remote.RestartCmd)
	} else {
		if b.projectInfo.Config.WildFlyMode == "standalone" {
			deployPath := filepath.Join(remote.WildFlyPath, "standalone", "deployments")
			fmt.Println("1. Copy artifact:")
			fmt.Printf("   scp %s %s@%s:%s\n", artifactPath, remote.User, remote.Host, deployPath)
			fmt.Println("\n2. Trigger deployment:")
			fmt.Printf("   ssh %s@%s \"touch %s/%s.dodeploy\"\n", remote.User, remote.Host, deployPath, artifactName)
		} else {
			// Domain mode
			serverGroup := b.projectInfo.Config.ServerGroup
			fmt.Println("1. Copy artifact to remote server:")
			fmt.Printf("   scp %s %s@%s:/tmp/\n", artifactPath, remote.User, remote.Host)
			fmt.Println("\n2. Deploy via jboss-cli:")
			jbossCli := filepath.Join(remote.WildFlyPath, "bin", "jboss-cli.sh")
			fmt.Printf("   ssh %s@%s \"%s --connect controller=localhost 'undeploy %s --server-groups=%s'\"\n",
				remote.User, remote.Host, jbossCli, artifactName, serverGroup)
			fmt.Printf("   ssh %s@%s \"%s --connect controller=localhost 'deploy /tmp/%s --server-groups=%s'\"\n",
				remote.User, remote.Host, jbossCli, artifactName, serverGroup)
		}
	}

	fmt.Println("\n3. Verify deployment:")
	logPath := filepath.Join(remote.WildFlyPath, "standalone", "log", "server.log")
	if b.projectInfo.Config.WildFlyMode == "domain" {
		logPath = filepath.Join(remote.WildFlyPath, "domain", "log", "server.log")
	}
	fmt.Printf("   ssh %s@%s \"tail -f %s\"\n", remote.User, remote.Host, logPath)
}

// isValidProfile checks if a profile is valid for the current project
func (b *Builder) isValidProfile(profile string) bool {
	if len(b.projectInfo.Config.AvailableProfiles) == 0 {
		// No profile restrictions
		return true
	}

	for _, validProfile := range b.projectInfo.Config.AvailableProfiles {
		if validProfile == profile {
			return true
		}
	}
	return false
}

// confirm asks the user for confirmation
func (b *Builder) confirm(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n%s [Y/n] ", question)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes"
}

// showRestartGuidance displays restart requirements based on config rules
func (b *Builder) showRestartGuidance() {
	// Find the main artifact
	targetDir := filepath.Join(b.projectInfo.ModulePath, "target")
	var artifactName string
	if entries, err := os.ReadDir(targetDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".war") {
				artifactName = name
				break
			}
			if strings.HasSuffix(name, ".jar") && !strings.Contains(name, "sources") && !strings.Contains(name, "javadoc") {
				artifactName = name
			}
		}
	}

	if artifactName == "" {
		return
	}

	severity, reason := b.determineRestartRequirement(artifactName)

	fmt.Println("\n" + strings.Repeat("━", 50))

	if severity == "none" {
		fmt.Println("ℹ️  NO RESTART NEEDED")
		fmt.Printf("Reason: %s\n", reason)
		return
	}

	if severity == "recommended" {
		fmt.Println("⚠️  RESTART RECOMMENDED")
		fmt.Printf("Reason: %s\n", reason)
	} else if severity == "required" {
		fmt.Println("⚠️  RESTART REQUIRED")
		fmt.Printf("Reason: %s\n", reason)
	}

	// Show restart commands
	fmt.Println("\nRestart command:")
	wildflyBin := filepath.Join(b.projectInfo.Config.WildFlyRoot, "bin")
	fmt.Printf("  cd %s\n", wildflyBin)
	fmt.Printf("  ./jboss-cli.sh --connect")
	if b.projectInfo.Config.WildFlyMode == "domain" {
		fmt.Printf(" controller=localhost")
	}
	fmt.Printf(" --command=\":shutdown\"\n")

	// Show alias if available
	if b.projectInfo.Name == "sinfomar" {
		fmt.Printf("  sin-wildfly\n")
	} else if b.projectInfo.Name == "mto" {
		fmt.Printf("  mto-wildfly\n")
	}
}

// determineRestartRequirement uses config rules to determine restart needs
func (b *Builder) determineRestartRequirement(artifactName string) (string, string) {
	// Check global module override
	if b.projectInfo.IsGlobalModule && b.cfg.RestartRules != nil && b.cfg.RestartRules.GlobalModule {
		return "required", "Global module modification"
	}

	// If no restart rules configured, fall back to basic checks
	if b.cfg.RestartRules == nil || len(b.cfg.RestartRules.Patterns) == 0 {
		return b.fallbackRestartCheck(artifactName)
	}

	// Check artifact name against patterns
	for _, pattern := range b.cfg.RestartRules.Patterns {
		matched, err := regexp.MatchString(pattern.Match, artifactName)
		if err != nil {
			// Skip invalid regex patterns
			continue
		}
		if matched {
			return pattern.Severity, pattern.Reason
		}
	}

	// No patterns matched, check WAR for hot-deployment
	if strings.HasSuffix(artifactName, ".war") {
		return "none", "WAR hot-deployment"
	}

	return "recommended", "Standard deployment (restart to ensure changes are loaded)"
}

// fallbackRestartCheck provides basic restart checks when no rules are configured
func (b *Builder) fallbackRestartCheck(artifactName string) (string, string) {
	if b.projectInfo.IsGlobalModule {
		return "required", "Global module modification"
	}

	if strings.HasSuffix(artifactName, ".jar") && strings.Contains(artifactName, "EJB") {
		return "recommended", "EJB implementation JAR"
	}

	if strings.HasSuffix(artifactName, ".war") {
		return "none", "WAR hot-deployment"
	}

	return "recommended", "Standard deployment (restart to ensure changes are loaded)"
}
