package cmd

import (
	"fmt"
	"os"

	embeddings_config "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/gemini"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/sources"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	appconfig "github.com/go-go-golems/glazed/pkg/config"
	"github.com/go-go-golems/pinocchio/pkg/cmds/cmdlayers"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const (
	jesusAppName     = "jesus"
	jesusEnvPrefix   = "JESUS"
	pinocchioAppName = "pinocchio"
)

// BuildCobraCommandWithServeMiddlewares builds a Cobra command with custom jesus middlewares
// that include profile support specifically for the jesus application.
func BuildCobraCommandWithServeMiddlewares(
	cmd cmds.Command,
	options ...cli.CobraOption,
) (*cobra.Command, error) {
	options_ := append([]cli.CobraOption{
		cli.WithParserConfig(cli.CobraParserConfig{
			AppName:         jesusAppName,
			MiddlewaresFunc: GetServeCommandMiddlewares,
		}),
		cli.WithCobraShortHelpSections(schema.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	}, options...)

	return cli.BuildCobraCommandFromCommand(cmd, options_...)
}

// GetServeCommandMiddlewares provides the source chain for jesus commands
// with proper profile support, configuration loading, and field precedence.
func GetServeCommandMiddlewares(
	parsedCommandSections *values.Values,
	cmd *cobra.Command,
	args []string,
) ([]sources.Middleware, error) {
	commandSettings := &cli.CommandSettings{}
	if err := parsedCommandSections.DecodeSectionInto(cli.CommandSettingsSlug, commandSettings); err != nil {
		return nil, err
	}

	profileSettings := &cli.ProfileSettings{}
	if err := parsedCommandSections.DecodeSectionInto(cli.ProfileSettingsSlug, profileSettings); err != nil {
		return nil, err
	}

	jesusConfigFiles, err := resolveConfigFiles(jesusAppName, commandSettings.ConfigFile)
	if err != nil {
		return nil, err
	}

	pinocchioConfigFiles, err := resolveConfigFiles(pinocchioAppName, "")
	if err != nil {
		return nil, err
	}

	middlewares_ := []sources.Middleware{
		sources.FromCobra(cmd,
			fields.WithSource("cobra"),
		),
		sources.FromArgs(args,
			fields.WithSource("arguments"),
		),
		sources.FromEnv(jesusEnvPrefix,
			fields.WithSource("env"),
		),
	}

	// Profile support with layered configuration: pinocchio first, then jesus overrides
	xdgConfigPath, err := os.UserConfigDir()
	if err != nil {
		log.Warn().Err(err).Msg("Could not get user config directory, using current directory")
		xdgConfigPath = "."
	}

	// Set up profile files: pinocchio as base, jesus as override
	pinocchioProfileFile := fmt.Sprintf("%s/pinocchio/profiles.yaml", xdgConfigPath)
	jesusProfileFile := fmt.Sprintf("%s/jesus/profiles.yaml", xdgConfigPath)

	// Use specified profile file or default to jesus
	targetProfileFile := profileSettings.ProfileFile
	if targetProfileFile == "" {
		targetProfileFile = jesusProfileFile
	}

	// Default to development profile for jesus
	profile := profileSettings.Profile
	if profile == "" {
		profile = "default"
	}

	middlewares_ = append(middlewares_,
		sources.GatherFlagsFromCustomProfiles(
			profile,
			sources.WithProfileFile(targetProfileFile),
			sources.WithProfileRequired(false), // Don't fail if profile doesn't exist
			sources.WithProfileParseOptions(
				fields.WithSource("jesus-profiles"),
				fields.WithMetadata(map[string]interface{}{
					"profileFile": targetProfileFile,
					"profile":     profile,
					"section":     "override",
				}),
			),
		),
		// Pinocchio profiles as base configuration
		sources.GatherFlagsFromProfiles(
			pinocchioProfileFile,
			pinocchioProfileFile,
			profile,
			"default",
			fields.WithSource("pinocchio-profiles"),
			fields.WithMetadata(map[string]interface{}{
				"profileFile": pinocchioProfileFile,
				"profile":     profile,
				"section":     "base",
			}),
		),
	)

	aiSectionMiddlewares := []sources.Middleware{}
	if len(pinocchioConfigFiles) > 0 {
		aiSectionMiddlewares = append(aiSectionMiddlewares,
			sources.FromFiles(pinocchioConfigFiles,
				sources.WithParseOptions(fields.WithSource("pinocchio-config"))),
		)
	}
	if len(jesusConfigFiles) > 0 {
		aiSectionMiddlewares = append(aiSectionMiddlewares,
			sources.FromFiles(jesusConfigFiles,
				sources.WithParseOptions(fields.WithSource("jesus-config"))),
		)
	}
	aiSectionMiddlewares = append(aiSectionMiddlewares,
		sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	)

	middlewares_ = append(middlewares_,
		sources.WrapWithWhitelistedSections(
			[]string{
				settings.AiChatSlug,
				settings.AiClientSlug,
				settings.AiInferenceSlug,
				openai.OpenAiChatSlug,
				claude.ClaudeChatSlug,
				gemini.GeminiChatSlug,
				cmdlayers.GeppettoHelpersSlug,
				embeddings_config.EmbeddingsSlug,
				cli.ProfileSettingsSlug,
			},
			aiSectionMiddlewares...,
		),
	)

	defaultSectionMiddlewares := []sources.Middleware{}
	if len(jesusConfigFiles) > 0 {
		defaultSectionMiddlewares = append(defaultSectionMiddlewares,
			sources.FromFiles(jesusConfigFiles,
				sources.WithParseOptions(fields.WithSource("jesus-config"))),
		)
	}
	defaultSectionMiddlewares = append(defaultSectionMiddlewares,
		sources.FromDefaults(fields.WithSource(fields.SourceDefaults)),
	)

	middlewares_ = append(middlewares_,
		sources.WrapWithWhitelistedSections(
			[]string{
				schema.DefaultSlug, // Include the default section for jesus settings
			},
			defaultSectionMiddlewares...,
		),
	)

	return middlewares_, nil
}

func resolveConfigFiles(appName string, explicit string) ([]string, error) {
	if appName == "" && explicit == "" {
		return nil, nil
	}

	path, err := appconfig.ResolveAppConfigPath(appName, explicit)
	if err != nil {
		return nil, err
	}

	if path == "" {
		return nil, nil
	}

	return []string{path}, nil
}
