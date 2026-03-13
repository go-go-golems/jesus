package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/jesus/pkg/engine"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// RunScriptsCmd represents the run-scripts command
type RunScriptsCmd struct {
	*cmds.CommandDescription
}

// RunScriptsSettings holds the configuration for the run-scripts command
type RunScriptsSettings struct {
	ScriptsDir string   `glazed:"scripts"`
	Files      []string `glazed:"files"`
}

// Ensure RunScriptsCmd implements BareCommand
var _ cmds.BareCommand = &RunScriptsCmd{}

// NewRunScriptsCmd creates a new run-scripts command
func NewRunScriptsCmd() (*RunScriptsCmd, error) {
	return &RunScriptsCmd{
		CommandDescription: cmds.NewCommandDescription(
			"run-scripts",
			cmds.WithShort("Execute JavaScript files without starting the web server"),
			cmds.WithLong(`Execute JavaScript files without starting the web server.

The command is useful for:
• Running test scripts
• Executing batch operations
• Testing route registration and runtime state
• Running standalone JavaScript with database bindings

Examples:
  run-scripts --scripts ./tests
  run-scripts --files test1.js,test2.js
  run-scripts --scripts ./jobs
  run-scripts --files seed.js,migrate.js`),
			cmds.WithFlags(
				fields.New(
					"scripts",
					fields.TypeString,
					fields.WithHelp("Directory containing JavaScript files to execute"),
					fields.WithShortFlag("s"),
					fields.WithDefault("./scripts"),
				),
				fields.New(
					"files",
					fields.TypeStringList,
					fields.WithHelp("Specific JavaScript files to execute (if not provided, all .js files in scripts directory)"),
					fields.WithShortFlag("f"),
				),
			),
		),
	}, nil
}

// Run executes the run-scripts command
func (cmd *RunScriptsCmd) Run(ctx context.Context, parsedValues *values.Values) error {
	// Parse settings
	var runSettings RunScriptsSettings
	if err := parsedValues.DecodeSectionInto(schema.DefaultSlug, &runSettings); err != nil {
		return errors.Wrap(err, "failed to parse settings")
	}

	log.Info().Str("scripts_dir", runSettings.ScriptsDir).Msg("Starting JavaScript script execution")

	// Initialize JavaScript engine with in-memory databases (since we don't need persistence for script execution)
	jsEngine := engine.NewEngine(":memory:", ":memory:")
	defer func() { _ = jsEngine.Close() }()

	// Determine which files to execute
	var filesToExecute []string

	if len(runSettings.Files) > 0 {
		// Execute specific files
		filesToExecute = runSettings.Files
		log.Info().Strs("files", runSettings.Files).Msg("Executing specific files")
	} else {
		// Execute all .js files in the scripts directory
		if _, err := os.Stat(runSettings.ScriptsDir); os.IsNotExist(err) {
			return errors.Errorf("scripts directory does not exist: %s", runSettings.ScriptsDir)
		}

		err := filepath.Walk(runSettings.ScriptsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && filepath.Ext(path) == ".js" {
				filesToExecute = append(filesToExecute, path)
			}
			return nil
		})
		if err != nil {
			return errors.Wrap(err, "failed to scan scripts directory")
		}

		log.Info().Int("file_count", len(filesToExecute)).Str("directory", runSettings.ScriptsDir).Msg("Found JavaScript files")
	}

	if len(filesToExecute) == 0 {
		log.Warn().Msg("No JavaScript files found to execute")
		return nil
	}

	// Execute each file
	for _, filePath := range filesToExecute {
		log.Info().Str("file", filePath).Msg("Executing JavaScript file")

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Error().Err(err).Str("file", filePath).Msg("Failed to read file")
			continue
		}

		// Execute the script and capture results
		result, err := jsEngine.ExecuteScript(string(content))
		if err != nil {
			log.Error().Err(err).Str("file", filePath).Msg("Failed to execute file")
			continue
		}

		log.Info().Str("file", filePath).Msg("Successfully executed JavaScript file")
		if result.Value != nil {
			log.Info().Interface("result", result.Value).Str("file", filePath).Msg("Script result")
		}
		if len(result.ConsoleLog) > 0 {
			for _, logLine := range result.ConsoleLog {
				log.Info().Str("console", logLine).Str("file", filePath).Msg("Script console output")
			}
		}
		if result.Error != nil {
			log.Error().Err(result.Error).Str("file", filePath).Msg("Script execution error")
		}
	}

	log.Info().Msg("JavaScript script execution completed")
	return nil
}
