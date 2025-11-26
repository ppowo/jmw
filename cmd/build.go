package cmd

import (
	"fmt"

	"github.com/ppowo/gmw/internal/builder"
	"github.com/ppowo/gmw/internal/config"
	"github.com/ppowo/gmw/internal/detector"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [PROFILE]",
	Short: "Build a Maven module",
	Long: `Build the current Maven module with the appropriate configuration.

Auto-detects the project (sinfomar/mto) from the current directory.

For sinfomar:
  gmw build        # Uses TEST profile (default)
  gmw build TEST   # Uses TEST profile
  gmw build PROD   # Uses PROD profile

For mto:
  gmw build        # Plain build, no profiles

The command will:
1. Detect which project you're in
2. Find the module from pom.xml
3. Show the Maven command it will run
4. Ask for confirmation
5. Execute the build`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Detect project and module from current directory
	cwd, err := detector.GetWorkingDirectory()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	projectInfo, err := detector.DetectProject(cwd, cfg)
	if err != nil {
		return fmt.Errorf("failed to detect project: %w", err)
	}

	// Get profile if specified
	var profile string
	if len(args) > 0 {
		profile = args[0]
	}

	// Build the module
	b := builder.NewBuilder(cfg, projectInfo)
	if err := b.Build(profile); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	return nil
}
