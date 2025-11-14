package cmd

import (
	"fmt"
	"os"

	embeddings_config "github.com/go-go-golems/geppetto/pkg/embeddings/config"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/claude"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings/openai"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/cmds/middlewares"
	"github.com/go-go-golems/glazed/pkg/cmds/parameters"
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
		cli.WithCobraShortHelpLayers(layers.DefaultSlug, cmdlayers.GeppettoHelpersSlug),
	}, options...)

	return cli.BuildCobraCommandFromCommand(cmd, options_...)
}

// GetServeCommandMiddlewares provides the middleware chain for jesus commands
// with proper profile support, configuration loading, and parameter precedence.
func GetServeCommandMiddlewares(
	parsedCommandLayers *layers.ParsedLayers,
	cmd *cobra.Command,
	args []string,
) ([]middlewares.Middleware, error) {
	commandSettings := &cli.CommandSettings{}
	err := parsedCommandLayers.InitializeStruct(cli.CommandSettingsSlug, commandSettings)
	if err != nil {
		return nil, err
	}

	profileSettings := &cli.ProfileSettings{}
	err = parsedCommandLayers.InitializeStruct(cli.ProfileSettingsSlug, profileSettings)
	if err != nil {
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

	middlewares_ := []middlewares.Middleware{
		middlewares.ParseFromCobraCommand(cmd,
			parameters.WithParseStepSource("cobra"),
		),
		middlewares.GatherArguments(args,
			parameters.WithParseStepSource("arguments"),
		),
		middlewares.UpdateFromEnv(jesusEnvPrefix,
			parameters.WithParseStepSource("env"),
		),
	}

	if commandSettings.LoadParametersFromFile != "" {
		middlewares_ = append(middlewares_,
			middlewares.LoadParametersFromFile(
				commandSettings.LoadParametersFromFile,
				middlewares.WithParseOptions(parameters.WithParseStepSource("command-settings-file")),
			))
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
		middlewares.GatherFlagsFromCustomProfiles(
			profile,
			middlewares.WithProfileFile(targetProfileFile),
			middlewares.WithProfileRequired(false), // Don't fail if profile doesn't exist
			middlewares.WithProfileParseOptions(
				parameters.WithParseStepSource("jesus-profiles"),
				parameters.WithParseStepMetadata(map[string]interface{}{
					"profileFile": targetProfileFile,
					"profile":     profile,
					"layer":       "override",
				}),
			),
		),
		// Pinocchio profiles as base configuration
		middlewares.GatherFlagsFromProfiles(
			pinocchioProfileFile,
			pinocchioProfileFile,
			profile,
			parameters.WithParseStepSource("pinocchio-profiles"),
			parameters.WithParseStepMetadata(map[string]interface{}{
				"profileFile": pinocchioProfileFile,
				"profile":     profile,
				"layer":       "base",
			}),
		),
	)

	aiLayerMiddlewares := []middlewares.Middleware{}
	if len(pinocchioConfigFiles) > 0 {
		aiLayerMiddlewares = append(aiLayerMiddlewares,
			middlewares.LoadParametersFromFiles(pinocchioConfigFiles,
				middlewares.WithParseOptions(parameters.WithParseStepSource("pinocchio-config"))),
		)
	}
	if len(jesusConfigFiles) > 0 {
		aiLayerMiddlewares = append(aiLayerMiddlewares,
			middlewares.LoadParametersFromFiles(jesusConfigFiles,
				middlewares.WithParseOptions(parameters.WithParseStepSource("jesus-config"))),
		)
	}
	aiLayerMiddlewares = append(aiLayerMiddlewares,
		middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
	)

	middlewares_ = append(middlewares_,
		middlewares.WrapWithWhitelistedLayers(
			[]string{
				settings.AiChatSlug,
				settings.AiClientSlug,
				openai.OpenAiChatSlug,
				claude.ClaudeChatSlug,
				cmdlayers.GeppettoHelpersSlug,
				embeddings_config.EmbeddingsSlug,
				cli.ProfileSettingsSlug,
			},
			aiLayerMiddlewares...,
		),
	)

	defaultLayerMiddlewares := []middlewares.Middleware{}
	if len(jesusConfigFiles) > 0 {
		defaultLayerMiddlewares = append(defaultLayerMiddlewares,
			middlewares.LoadParametersFromFiles(jesusConfigFiles,
				middlewares.WithParseOptions(parameters.WithParseStepSource("jesus-config"))),
		)
	}
	defaultLayerMiddlewares = append(defaultLayerMiddlewares,
		middlewares.SetFromDefaults(parameters.WithParseStepSource("defaults")),
	)

	middlewares_ = append(middlewares_,
		middlewares.WrapWithWhitelistedLayers(
			[]string{
				layers.DefaultSlug, // Include the default layer for jesus settings
			},
			defaultLayerMiddlewares...,
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
