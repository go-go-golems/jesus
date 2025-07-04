package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/js"
	"github.com/rs/zerolog/log"
)

// setupBindings configures JavaScript bindings for the runtime
func (e *Engine) setupBindings() {
	// Handler registration
	if err := e.rt.Set("registerHandler", e.registerHandler); err != nil {
		log.Error().Err(err).Msg("Failed to set registerHandler binding")
	}
	if err := e.rt.Set("registerFile", e.registerFile); err != nil {
		log.Error().Err(err).Msg("Failed to set registerFile binding")
	}

	// HTTP utilities and constants
	e.setupHTTPUtilities()

	// HTTP request bindings
	e.setupHTTPBindings()

	// Console logging
	if err := e.rt.Set("console", map[string]interface{}{
		"log":   e.consoleLog,
		"error": e.consoleError,
		"info":  e.consoleInfo,
		"warn":  e.consoleWarn,
		"debug": e.consoleDebug,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to set console binding")
	}

	// Basic utilities
	if err := e.rt.Set("JSON", map[string]interface{}{
		"stringify": e.jsonStringify,
		"parse":     e.jsonParse,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to set JSON binding")
	}

	// Global state object for persistence across script executions
	if _, err := e.rt.RunString(`
		if (typeof globalState === 'undefined') {
			globalState = {};
		}
	`); err != nil {
		log.Error().Err(err).Msg("Failed to initialize globalState")
	}

	// Setup Geppetto JavaScript APIs
	e.setupGeppettoAPIs()

	log.Debug().Msg("JavaScript bindings configured")
}

// consoleLog provides console.log functionality
func (e *Engine) consoleLog(args ...interface{}) {
	log.Info().Interface("args", args).Msg("JS console.log")
	fmt.Fprint(os.Stderr, "[JS] ")
	for i, arg := range args {
		if i > 0 {
			fmt.Fprint(os.Stderr, " ")
		}
		fmt.Fprint(os.Stderr, arg)
	}
	fmt.Fprintln(os.Stderr)

	// Also log to request logger if we have a current request
	if e.currentReqID != "" {
		message := fmt.Sprintf("%v", args)
		e.reqLogger.AddLog(e.currentReqID, "log", message, args)
	}
}

// consoleError provides console.error functionality
func (e *Engine) consoleError(args ...interface{}) {
	log.Error().Interface("args", args).Msg("JS console.error")
	fmt.Fprint(os.Stderr, "[JS ERROR] ")
	for i, arg := range args {
		if i > 0 {
			fmt.Fprint(os.Stderr, " ")
		}
		fmt.Fprint(os.Stderr, arg)
	}
	fmt.Fprintln(os.Stderr)

	// Also log to request logger if we have a current request
	if e.currentReqID != "" {
		message := fmt.Sprintf("%v", args)
		e.reqLogger.AddLog(e.currentReqID, "error", message, args)
	}
}

// consoleInfo provides console.info functionality
func (e *Engine) consoleInfo(args ...interface{}) {
	log.Info().Interface("args", args).Msg("JS console.info")
	fmt.Fprint(os.Stderr, "[JS INFO] ")
	for i, arg := range args {
		if i > 0 {
			fmt.Fprint(os.Stderr, " ")
		}
		fmt.Fprint(os.Stderr, arg)
	}
	fmt.Fprintln(os.Stderr)

	// Also log to request logger if we have a current request
	if e.currentReqID != "" {
		message := fmt.Sprintf("%v", args)
		e.reqLogger.AddLog(e.currentReqID, "info", message, args)
	}
}

// consoleWarn provides console.warn functionality
func (e *Engine) consoleWarn(args ...interface{}) {
	log.Warn().Interface("args", args).Msg("JS console.warn")
	fmt.Fprint(os.Stderr, "[JS WARN] ")
	for i, arg := range args {
		if i > 0 {
			fmt.Fprint(os.Stderr, " ")
		}
		fmt.Fprint(os.Stderr, arg)
	}
	fmt.Fprintln(os.Stderr)

	// Also log to request logger if we have a current request
	if e.currentReqID != "" {
		message := fmt.Sprintf("%v", args)
		e.reqLogger.AddLog(e.currentReqID, "warn", message, args)
	}
}

// consoleDebug provides console.debug functionality
func (e *Engine) consoleDebug(args ...interface{}) {
	log.Debug().Interface("args", args).Msg("JS console.debug")
	fmt.Fprint(os.Stderr, "[JS DEBUG] ")
	for i, arg := range args {
		if i > 0 {
			fmt.Fprint(os.Stderr, " ")
		}
		fmt.Fprint(os.Stderr, arg)
	}
	fmt.Fprintln(os.Stderr)

	// Also log to request logger if we have a current request
	if e.currentReqID != "" {
		message := fmt.Sprintf("%v", args)
		e.reqLogger.AddLog(e.currentReqID, "debug", message, args)
	}
}

// jsonStringify provides JSON.stringify functionality
func (e *Engine) jsonStringify(obj interface{}) string {
	data, err := json.Marshal(obj)
	if err != nil {
		return "null"
	}
	return string(data)
}

// jsonParse provides JSON.parse functionality
func (e *Engine) jsonParse(str string) interface{} {
	var result interface{}
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		panic(e.rt.NewGoError(err))
	}
	return result
}

// ConsoleCapture holds original console functions and captured output
type ConsoleCapture struct {
	Log   func(...interface{})
	Error func(...interface{})
	Info  func(...interface{})
	Warn  func(...interface{})
	Debug func(...interface{})
}

// captureConsole replaces console functions to capture output
func (e *Engine) captureConsole(result *EvalResult) *ConsoleCapture {
	// Store original console functions
	original := &ConsoleCapture{
		Log:   e.consoleLog,
		Error: e.consoleError,
		Info:  e.consoleInfo,
		Warn:  e.consoleWarn,
		Debug: e.consoleDebug,
	}

	// Create capturing versions
	if err := e.rt.Set("console", map[string]interface{}{
		"log":   func(args ...interface{}) { e.captureConsoleOutput(result, "log", args...) },
		"error": func(args ...interface{}) { e.captureConsoleOutput(result, "error", args...) },
		"info":  func(args ...interface{}) { e.captureConsoleOutput(result, "info", args...) },
		"warn":  func(args ...interface{}) { e.captureConsoleOutput(result, "warn", args...) },
		"debug": func(args ...interface{}) { e.captureConsoleOutput(result, "debug", args...) },
	}); err != nil {
		log.Error().Err(err).Msg("Failed to set console capture binding")
	}

	return original
}

// restoreConsole restores original console functions
func (e *Engine) restoreConsole(original *ConsoleCapture) {
	if err := e.rt.Set("console", map[string]interface{}{
		"log":   original.Log,
		"error": original.Error,
		"info":  original.Info,
		"warn":  original.Warn,
		"debug": original.Debug,
	}); err != nil {
		log.Error().Err(err).Msg("Failed to restore console binding")
	}
}

// captureConsoleOutput captures console output to the result
func (e *Engine) captureConsoleOutput(result *EvalResult, level string, args ...interface{}) {
	var parts []string
	for _, arg := range args {
		parts = append(parts, fmt.Sprint(arg))
	}
	output := fmt.Sprintf("[%s] %s", level, strings.Join(parts, " "))
	result.ConsoleLog = append(result.ConsoleLog, output)

	// Also call the original console function for logging
	switch level {
	case "log":
		e.consoleLog(args...)
	case "error":
		e.consoleError(args...)
	case "info":
		e.consoleInfo(args...)
	case "warn":
		e.consoleWarn(args...)
	case "debug":
		e.consoleDebug(args...)
	}
}

// setupGeppettoAPIs configures Geppetto JavaScript APIs (Conversation, Embeddings, Steps, ChatStepFactory)
func (e *Engine) setupGeppettoAPIs() {
	log.Debug().Msg("Setting up Geppetto JavaScript APIs")

	// Register Conversation API
	if err := js.RegisterConversation(e.rt); err != nil {
		log.Error().Err(err).Msg("Failed to register Conversation API")
	} else {
		log.Debug().Msg("Conversation API registered")
	}

	// Register ChatStepFactory
	// if err := js.RegisterFactory(e.rt, e.loop, e.stepSettings); err != nil {
	// 	log.Error().Err(err).Msg("Failed to register ChatStepFactory")
	// } else {
	// 	log.Debug().Msg("ChatStepFactory registered")
	// }

	// TODO: Register Embeddings API when available
	// This requires an embeddings provider which we don't have configured yet
	// if err := js.RegisterEmbeddings(e.rt, "embeddings", embeddingsProvider, e.loop); err != nil {
	//     log.Error().Err(err).Msg("Failed to register Embeddings API")
	// } else {
	//     log.Debug().Msg("Embeddings API registered")
	// }

	// TODO: Register additional Steps API when needed
	// Steps are typically registered on a per-use basis

	log.Debug().Msg("Geppetto JavaScript APIs setup complete")
}

// setupGeppettoBindings sets up only the Geppetto API bindings
func (e *Engine) setupGeppettoBindings() error {
	log.Debug().Msg("Setting up Geppetto JavaScript APIs")

	// Register Conversation API
	if err := js.RegisterConversation(e.rt); err != nil {
		log.Error().Err(err).Msg("Failed to register Conversation API")
		return err
	}
	log.Debug().Msg("Conversation API registered")

	// Register ChatStepFactory
	// if err := js.RegisterFactory(e.rt, e.loop, e.stepSettings); err != nil {
	// 	log.Error().Err(err).Msg("Failed to register ChatStepFactory")
	// 	return err
	// }
	log.Debug().Msg("ChatStepFactory registered")

	log.Debug().Msg("Geppetto JavaScript APIs setup complete")
	return nil
}
