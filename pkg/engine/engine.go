package engine

import (
	"net/http"
	"os"
	"sync"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	gogogojamodules "github.com/go-go-golems/go-go-goja/modules"
	databasemod "github.com/go-go-golems/go-go-goja/modules/database"
	"github.com/go-go-golems/jesus/pkg/repository"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
)

// Engine wraps the JavaScript runtime and data repositories
type Engine struct {
	rt             *goja.Runtime
	loop           *eventloop.EventLoop         // Event loop for async operations
	repos          repository.RepositoryManager // Repository manager for data access
	jobs           chan EvalJob
	handlers       map[string]map[string]*HandlerInfo // [path][method] -> handler info
	files          map[string]goja.Callable           // [path] -> file handler
	mu             sync.RWMutex
	reqLogger      *RequestLogger         // Request logger for admin interface
	currentReqID   string                 // Track current request ID for logging
	stepSettings   *settings.StepSettings // Settings for AI steps
	moduleRegistry *gogogojamodules.Registry
}

// HandlerInfo contains handler function and metadata
type HandlerInfo struct {
	Fn          goja.Callable          // JavaScript function
	ContentType string                 // MIME type override
	Options     map[string]interface{} // Handler options (middleware, auth, etc.)
}

// EvalJob represents a JavaScript evaluation job
type EvalJob struct {
	Handler   *HandlerInfo        // pre-registered handler info (nil for direct code execution)
	Code      string              // JavaScript code to execute
	W         http.ResponseWriter // response writer
	R         *http.Request       // request
	Done      chan error          // completion signal
	Result    chan *EvalResult    // result channel for capturing execution results
	SessionID string              // session identifier for tracking
	Source    string              // source of execution ('api', 'mcp', 'file')
}

// EvalResult contains the result of JavaScript execution
type EvalResult struct {
	Value      interface{} `json:"value"`           // The actual result value
	ConsoleLog []string    `json:"consoleLog"`      // Captured console output
	Error      error       `json:"error,omitempty"` // Execution error if any
}

// NewEngine creates a new JavaScript engine with separate application and system databases
func NewEngine(appDBPath, systemDBPath string) *Engine {
	log.Debug().Str("appDatabase", appDBPath).Str("systemDatabase", systemDBPath).Msg("Creating new JavaScript engine")

	// Create event loop for async operations
	loop := eventloop.NewEventLoop()
	log.Debug().Msg("Event loop created")

	rt := goja.New()
	log.Debug().Msg("Goja runtime created")

	moduleRegistry := gogogojamodules.DefaultRegistry
	gojaRegistry := require.NewRegistry()
	moduleRegistry.Enable(gojaRegistry)
	gojaRegistry.Enable(rt)

	dbModule, ok := moduleRegistry.GetModule("database").(*databasemod.DBModule)
	if !ok || dbModule == nil {
		log.Fatal().Msg("Database module not found or is not of type *databasemod.DBModule")
	}
	if err := dbModule.Configure("sqlite3", appDBPath); err != nil {
		log.Fatal().Err(err).Msg("Failed to configure database module")
	}

	// Set up field name mapper to convert Go method names to JavaScript-style names
	rt.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	// Create repository manager for system operations (system database)
	repos, err := repository.NewSQLiteRepositoryManager(systemDBPath)
	if err != nil {
		log.Fatal().Err(err).Str("database", systemDBPath).Msg("Failed to create repository manager")
	}
	log.Debug().Str("database", systemDBPath).Msg("System database repository manager created")

	// Initialize AI step settings
	stepSettings, err := settings.NewStepSettings()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create step settings")
	}
	log.Debug().Msg("AI step settings initialized")

	e := &Engine{
		rt:             rt,
		loop:           loop,
		repos:          repos,
		jobs:           make(chan EvalJob, 1024),
		handlers:       make(map[string]map[string]*HandlerInfo),
		files:          make(map[string]goja.Callable),
		reqLogger:      NewRequestLogger(100), // Keep last 100 requests
		stepSettings:   stepSettings,
		moduleRegistry: moduleRegistry,
	}
	log.Debug().Msg("Engine struct initialized")

	// Start the event loop
	loop.Start()
	log.Debug().Msg("Event loop started")

	// Setup JavaScript bindings
	log.Debug().Msg("Setting up JavaScript bindings")
	e.setupBindings()
	log.Debug().Msg("JavaScript bindings setup complete")

	if _, err := rt.RunString(`const db = require('database');`); err != nil {
		log.Fatal().Err(err).Msg("Failed to bind db to global scope")
	}

	// Log runtime state after bindings setup
	e.logJavaScriptRuntimeState("after-bindings-setup")

	log.Debug().Msg("JavaScript engine initialized with repository pattern and Geppetto API")
	return e
}

// UpdateStepSettings updates the AI step settings for the engine
func (e *Engine) UpdateStepSettings(stepSettings *settings.StepSettings) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stepSettings = stepSettings
	log.Debug().Msg("Step settings updated")

	// Re-register Geppetto APIs with new settings
	return e.setupGeppettoBindings()
}

// ExecuteScript executes JavaScript code and returns the result with console output
func (e *Engine) ExecuteScript(code string) (*EvalResult, error) {
	return e.executeCodeWithResult(code)
}

// Init loads and executes a bootstrap JavaScript file
func (e *Engine) Init(filename string) error {
	log.Debug().Str("file", filename).Msg("Initializing JavaScript engine with bootstrap file")

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Debug().Str("file", filename).Msg("Bootstrap file doesn't exist, creating default")

		// Create default bootstrap file
		bootstrap := `// Initialize global counter (safe for re-execution)
if (!globalState.counter) {
    globalState.counter = 0;
}

// Basic routes using Express.js API
app.get("/", (req, res) => {
    res.send("JS playground online with Geppetto APIs");
});

app.get("/health", (req, res) => {
    res.json({ok: true, counter: globalState.counter});
});

app.post("/counter", (req, res) => {
    res.json({count: ++globalState.counter});
});

// Example Geppetto API usage route
app.get("/geppetto-demo", (req, res) => {
    try {
        // Create a new conversation
        const conv = new Conversation();
        
        // Add a simple message (note: using lowercase method names due to field name mapper)
        const msgId = conv.addMessage("user", "Hello, Geppetto!");
        console.log("Added message with ID:", msgId);
        
        // Get conversation as single prompt
        const prompt = conv.getSinglePrompt();
        
        res.json({
            success: true,
            messageId: msgId,
            prompt: prompt,
            conversationAPI: "Available",
            chatFactory: typeof ChatStepFactory !== 'undefined' ? "Available" : "Not Available"
        });
    } catch (error) {
        console.error("Geppetto demo error:", error);
        res.status(500).json({
            success: false,
            error: error.message
        });
    }
});

console.log("Bootstrap complete - server ready with Geppetto APIs");`

		if err := os.WriteFile(filename, []byte(bootstrap), 0644); err == nil {
			log.Debug().Str("file", filename).Msg("Created default bootstrap file")
			return e.executeCode(bootstrap)
		}
		log.Error().Err(err).Str("file", filename).Msg("Failed to create bootstrap file")
		return err
	}

	log.Debug().Str("file", filename).Msg("Loading existing bootstrap file")
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Error().Err(err).Str("file", filename).Msg("Failed to read bootstrap file")
		return err
	}

	log.Debug().Str("file", filename).Int("size", len(data)).Msg("Bootstrap file loaded, executing JavaScript")
	err = e.executeCode(string(data))
	if err != nil {
		log.Error().Err(err).Str("file", filename).Msg("Failed to execute bootstrap file")
	} else {
		log.Info().Str("file", filename).Msg("Bootstrap file executed successfully")
	}
	return err
}

// GetHandler returns a registered HTTP handler, supporting path parameters
func (e *Engine) GetHandler(method, path string) (*HandlerInfo, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	log.Debug().Str("method", method).Str("path", path).Msg("Looking for handler")

	// First try exact match
	if methods, exists := e.handlers[path]; exists {
		log.Debug().Str("path", path).Msg("Found exact path match")
		if handler, exists := methods[method]; exists {
			log.Debug().Str("method", method).Str("path", path).Msg("Found exact handler match")
			return handler, true
		} else {
			log.Debug().Str("method", method).Str("path", path).Interface("availableMethods", getMapKeys(methods)).Msg("Path exists but method not found")
		}
	}

	// Try pattern matching for path parameters
	log.Debug().Str("method", method).Str("path", path).Msg("Trying pattern matching for path parameters")
	for pattern, methods := range e.handlers {
		if handler, exists := methods[method]; exists {
			if pathMatches(pattern, path) {
				log.Debug().Str("method", method).Str("path", path).Str("pattern", pattern).Msg("Found pattern match")
				return handler, true
			}
		}
	}

	log.Debug().Str("method", method).Str("path", path).Int("totalHandlers", len(e.handlers)).Msg("No handler found")
	return nil, false
}

// Helper function to get map keys for logging
func getMapKeys(m map[string]*HandlerInfo) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GetFileHandler returns a registered file handler
func (e *Engine) GetFileHandler(path string) (goja.Callable, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	handler, exists := e.files[path]
	return handler, exists
}

// SubmitJob submits a job to the dispatcher
func (e *Engine) SubmitJob(job EvalJob) {
	e.jobs <- job
}

// GetRequestLogger returns the request logger for admin interface
func (e *Engine) GetRequestLogger() *RequestLogger {
	return e.reqLogger
}

// GetRepositoryManager returns the repository manager
func (e *Engine) GetRepositoryManager() repository.RepositoryManager {
	return e.repos
}

// GetModuleRegistry returns the go-go-goja module registry.
func (e *Engine) GetModuleRegistry() *gogogojamodules.Registry {
	return e.moduleRegistry
}

// executeCode executes JavaScript code directly in the global scope
func (e *Engine) executeCode(code string) error {
	log.Debug().Str("code", code).Msg("Executing JavaScript code")

	// Log runtime state before execution
	e.logJavaScriptRuntimeState("before-execution")

	_, err := e.rt.RunString(code)
	if err != nil {
		log.Error().Err(err).Str("code", code).Msg("JavaScript execution error")
	} else {
		log.Debug().Str("code", code).Msg("JavaScript code executed successfully")
	}

	// Log runtime state after execution
	e.logJavaScriptRuntimeState("after-execution")

	return err
}

// executeCodeWithResult executes JavaScript code and captures the result and console output
func (e *Engine) executeCodeWithResult(code string) (*EvalResult, error) {
	result := &EvalResult{
		ConsoleLog: []string{},
	}

	// Temporarily capture console output
	originalConsole := e.captureConsole(result)
	defer e.restoreConsole(originalConsole)

	log.Debug().Str("code", code).Msg("Executing JavaScript code with result capture")

	// Log runtime state before execution
	e.logJavaScriptRuntimeState("before-execution-with-result")

	value, err := e.rt.RunString(code)
	if err != nil {
		log.Error().Err(err).Str("code", code).Msg("JavaScript execution error with result capture")
		result.Error = err
		return result, err
	}

	// Export the result to a Go-friendly format
	if value != nil && !goja.IsUndefined(value) {
		result.Value = value.Export()
		log.Debug().Interface("resultValue", result.Value).Msg("JavaScript execution result captured")
	} else {
		log.Debug().Msg("JavaScript execution returned undefined or null")
	}

	log.Debug().Int("consoleLogCount", len(result.ConsoleLog)).Msg("Console output captured")

	// Log runtime state after execution
	e.logJavaScriptRuntimeState("after-execution-with-result")

	return result, nil
}

// logJavaScriptRuntimeState logs the current state of the JavaScript runtime for debugging
func (e *Engine) logJavaScriptRuntimeState(context string) {
	log.Debug().Str("context", context).Msg("Logging JavaScript runtime state")

	// Check if app object exists and has methods
	appValue := e.rt.Get("app")
	if appValue != nil && !goja.IsUndefined(appValue) {
		log.Debug().Str("context", context).Str("appType", appValue.String()).Msg("app object exists in runtime")

		// Try to get app.get method
		if appObj := appValue.ToObject(e.rt); appObj != nil {
			getMethod := appObj.Get("get")
			if getMethod != nil && !goja.IsUndefined(getMethod) {
				log.Debug().Str("context", context).Str("getMethodType", getMethod.String()).Msg("app.get method exists")
			} else {
				log.Debug().Str("context", context).Msg("app.get method is undefined")
			}
		}
	} else {
		log.Debug().Str("context", context).Msg("app object is undefined in runtime")
	}

	// Check globalState
	globalStateValue := e.rt.Get("globalState")
	if globalStateValue != nil && !goja.IsUndefined(globalStateValue) {
		log.Debug().Str("context", context).Str("globalStateType", globalStateValue.String()).Msg("globalState exists in runtime")
	} else {
		log.Debug().Str("context", context).Msg("globalState is undefined in runtime")
	}

	// Check console
	consoleValue := e.rt.Get("console")
	if consoleValue != nil && !goja.IsUndefined(consoleValue) {
		log.Debug().Str("context", context).Str("consoleType", consoleValue.String()).Msg("console exists in runtime")
	} else {
		log.Debug().Str("context", context).Msg("console is undefined in runtime")
	}
}

// GetGlobalState returns the current globalState object as JSON string
func (e *Engine) GetGlobalState() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	globalStateValue := e.rt.Get("globalState")
	if globalStateValue == nil || goja.IsUndefined(globalStateValue) {
		return "{}"
	}

	return e.stringifyJSValue(globalStateValue)
}

// SetGlobalState sets the globalState object from a JSON string
func (e *Engine) SetGlobalState(jsonData string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Parse JSON and set globalState
	code := "globalState = " + jsonData
	_, err := e.rt.RunString(code)
	if err != nil {
		log.Error().Err(err).Str("json", jsonData).Msg("Failed to set globalState")
		return err
	}

	log.Debug().Str("json", jsonData).Msg("GlobalState updated")
	return nil
}

// stringifyJSValue uses JavaScript's JSON.stringify to convert a Goja value to a JSON string
func (e *Engine) stringifyJSValue(value goja.Value) string {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return "null"
	}

	// Get JSON.stringify function from the runtime
	jsonObj := e.rt.Get("JSON")
	if jsonObj == nil || goja.IsUndefined(jsonObj) {
		// Fallback to Go's string representation if JSON is not available
		return value.String()
	}

	jsonObjRef := jsonObj.ToObject(e.rt)
	if jsonObjRef == nil {
		return value.String()
	}

	stringifyFn := jsonObjRef.Get("stringify")
	if stringifyFn == nil || goja.IsUndefined(stringifyFn) {
		return value.String()
	}

	// Call JSON.stringify(value, null, 2) for pretty printing
	stringifyCallable, ok := goja.AssertFunction(stringifyFn)
	if !ok {
		return value.String()
	}

	result, err := stringifyCallable(jsonObj, value, goja.Null(), e.rt.ToValue(2))
	if err != nil {
		log.Debug().Err(err).Msg("Failed to stringify JavaScript value, falling back to string representation")
		return value.String()
	}

	if result == nil || goja.IsUndefined(result) {
		return "undefined"
	}

	return result.String()
}

// Close gracefully shuts down the engine
func (e *Engine) Close() error {
	log.Debug().Msg("Shutting down JavaScript engine")

	// Stop the event loop
	if e.loop != nil {
		e.loop.Stop()
		log.Debug().Msg("Event loop stopped")
	}

	// Close repository manager
	if e.repos != nil {
		dbModule, ok := gogogojamodules.GetModule("database").(*databasemod.DBModule)
		if ok && dbModule != nil {
			if err := dbModule.Close(); err != nil {
				log.Error().Err(err).Msg("Failed to close database module")
			}
		}

		if err := e.repos.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close repository manager")
			return err
		}
		log.Debug().Msg("Repository manager closed")
	}

	log.Debug().Msg("JavaScript engine shutdown complete")
	return nil
}
