# Moving JavaScript Libraries from Jesus to Reusable go-go-goja Modules

## Executive Summary

This report analyzes the current JavaScript libraries embedded in the Jesus engine and provides a detailed plan for extracting them into reusable modules in the go-go-goja project. The goal is to make these JavaScript APIs (database, HTTP server, console, etc.) available to any goja runtime, not just Jesus.

## Current State Analysis

### JavaScript Libraries Currently in Jesus

The Jesus engine (`jesus/pkg/engine/`) currently implements several JavaScript libraries directly:

#### 1. Database Module (`bindings.go`)
- **Functions**: `db.query()`, `db.exec()`
- **Features**: 
  - SQLite integration with parameter binding
  - Automatic result conversion to JavaScript objects
  - Request logging integration
  - Error handling and transaction support
- **Dependencies**: `database/sql`, `github.com/mattn/go-sqlite3`

#### 2. HTTP Client Module (`http_bindings.go`)
- **Functions**: `fetch()`, `HTTP.get()`, `HTTP.post()`, etc.
- **Features**:
  - Modern fetch-like API
  - HTTP method shortcuts
  - Request/response transformation
  - Query parameter handling
  - JSON body parsing
  - Timeout support
- **Dependencies**: `net/http`, `encoding/json`

#### 3. Express.js-like Server Module (`handlers.go`)
- **Functions**: `app.get()`, `app.post()`, `app.put()`, `app.delete()`, `app.patch()`, `app.use()`
- **Features**:
  - Route registration with path parameters
  - Express.js compatible request/response objects
  - HTTP status code constants
  - Response methods: `res.json()`, `res.send()`, `res.status()`, `res.redirect()`, etc.
  - Cookie handling
  - Header management
- **Dependencies**: `net/http`, `github.com/dop251/goja`

#### 4. Console Module (`bindings.go`)
- **Functions**: `console.log()`, `console.error()`, `console.info()`, `console.warn()`, `console.debug()`
- **Features**:
  - Multi-level logging
  - Request-scoped log capture
  - Integration with zerolog
  - Console output capture for result analysis
- **Dependencies**: `github.com/rs/zerolog`

#### 5. JSON Module (`bindings.go`)
- **Functions**: `JSON.stringify()`, `JSON.parse()`
- **Features**:
  - JavaScript-compatible JSON handling
  - Error handling with goja exceptions
- **Dependencies**: `encoding/json`

#### 6. Global State Module (`bindings.go`)
- **Features**:
  - Persistent `globalState` object
  - Cross-execution state preservation
  - JSON serialization support

### Jesus-Specific Components (Not Suitable for Extraction)

#### 1. Request Logger (`request_logger.go`)
- **Purpose**: HTTP request logging and debugging
- **Features**: Request/response capture, database operation logging, real-time monitoring
- **Jesus-specific**: Tightly coupled to Jesus's web server architecture

#### 2. Job Dispatcher (`dispatcher.go`)
- **Purpose**: Asynchronous JavaScript execution
- **Features**: Job queuing, error handling, result capture
- **Jesus-specific**: Part of Jesus's HTTP server architecture

#### 3. Repository Integration (`engine.go`)
- **Purpose**: Integration with Jesus's data layer
- **Features**: Execution logging, session management
- **Jesus-specific**: Uses Jesus's repository pattern

#### 4. Geppetto API Integration (`bindings.go`)
- **Purpose**: AI/LLM integration
- **Features**: Conversation API, ChatStepFactory
- **Jesus-specific**: Requires Geppetto dependencies and configuration

## go-go-goja Current Architecture

### Module System Structure
- **Registry**: `modules/common.go` - Central module registration system
- **Interface**: `NativeModule` interface with `Name()` and `Loader()` methods
- **Auto-discovery**: Modules register themselves via `init()` functions
- **Integration**: `engine/runtime.go` enables all registered modules

### Existing Modules
1. **fs**: Basic file system operations (`readFileSync`, `writeFileSync`)
2. **exec**: Command execution wrapper (`run`)

### Module Pattern
```go
type m struct{}

var _ modules.NativeModule = (*m)(nil)

func (m) Name() string { return "module-name" }

func (m) Loader(vm *goja.Runtime, moduleObj *goja.Object) {
    exports := moduleObj.Get("exports").(*goja.Object)
    exports.Set("functionName", goFunction)
}

func init() { modules.Register(&m{}) }
```

## Migration Plan

### Phase 1: Core Utility Modules (Low Complexity)

#### 1.1 Console Module
**Target**: `go-go-goja/modules/console/console.go`

**Features to Extract**:
- Multi-level logging (`log`, `error`, `info`, `warn`, `debug`)
- Configurable output destinations
- Optional structured logging integration

**Dependencies**: 
- Core: None (use `fmt` and `os`)
- Optional: `github.com/rs/zerolog` for structured logging

**Interface**:
```javascript
const console = require("console");
console.log("message");
console.error("error message");
// etc.
```

**Implementation Considerations**:
- Remove Jesus-specific request logging integration
- Make logging destination configurable
- Support both simple and structured logging modes

#### 1.2 JSON Module
**Target**: `go-go-goja/modules/json/json.go`

**Features to Extract**:
- `JSON.stringify()` and `JSON.parse()`
- Error handling with goja exceptions
- Pretty printing support

**Dependencies**: `encoding/json`

**Interface**:
```javascript
const JSON = require("json");
const str = JSON.stringify({key: "value"});
const obj = JSON.parse(str);
```

### Phase 2: HTTP Client Module (Medium Complexity)

#### 2.1 HTTP Client Module
**Target**: `go-go-goja/modules/http-client/http-client.go`

**Features to Extract**:
- Modern `fetch()` API
- HTTP method shortcuts (`GET`, `POST`, etc.)
- Request/response transformation
- Timeout and header support
- JSON body handling

**Dependencies**: `net/http`, `encoding/json`

**Interface**:
```javascript
const { fetch, HTTP } = require("http-client");

// Modern fetch API
const response = await fetch("https://api.example.com/data");
const data = await response.json();

// Method shortcuts
const result = HTTP.get("https://api.example.com/users");
```

**Implementation Considerations**:
- Remove Jesus-specific request logging
- Make HTTP client configurable (timeouts, etc.)
- Support both synchronous and asynchronous patterns

### Phase 3: Database Module (Medium-High Complexity)

#### 3.1 Database Module
**Target**: `go-go-goja/modules/database/database.go`

**Features to Extract**:
- `db.query()` and `db.exec()` functions
- Parameter binding and SQL injection protection
- Result set conversion to JavaScript objects
- Transaction support
- Multiple database driver support

**Dependencies**: `database/sql`, optional drivers (`github.com/mattn/go-sqlite3`, etc.)

**Interface**:
```javascript
const db = require("database");

// Configure database connection
db.configure("sqlite3", "path/to/database.db");

// Query operations
const users = db.query("SELECT * FROM users WHERE active = ?", [true]);

// Exec operations
const result = db.exec("INSERT INTO users (name, email) VALUES (?, ?)", ["John", "john@example.com"]);
```

**Implementation Considerations**:
- Make database connection configurable per module instance
- Support multiple database drivers
- Remove Jesus-specific logging integration
- Add connection pooling and management
- Support both synchronous and asynchronous operations

### Phase 4: Express-like Server Module (High Complexity)

#### 4.1 Express Server Module
**Target**: `go-go-goja/modules/express/express.go`

**Features to Extract**:
- Route registration (`app.get`, `app.post`, etc.)
- Express.js compatible request/response objects
- Path parameter parsing
- Middleware support
- HTTP status code constants

**Dependencies**: `net/http`, `github.com/gorilla/mux` (optional)

**Interface**:
```javascript
const express = require("express");
const app = express();

app.get("/users/:id", (req, res) => {
    const userId = req.params.id;
    res.json({ id: userId, name: "John Doe" });
});

app.listen(8080);
```

**Implementation Considerations**:
- Decouple from Jesus's job dispatcher system
- Make HTTP server lifecycle manageable from JavaScript
- Support both embedded and standalone server modes
- Provide configurable routing and middleware systems
- Remove Jesus-specific handler registration

## Technical Challenges and Solutions

### Challenge 1: Dependency Management
**Problem**: Jesus uses many dependencies that go-go-goja shouldn't require
**Solution**: 
- Use interface-based design for optional dependencies
- Provide default implementations using standard library
- Allow dependency injection for advanced features

### Challenge 2: Asynchronous Operations
**Problem**: Jesus uses event loops and job dispatchers for async operations
**Solution**:
- Implement Promise-based APIs using `goja.NewPromise()`
- Use `goja_nodejs/eventloop` for async operations
- Provide both sync and async variants where appropriate

### Challenge 3: Configuration and State Management
**Problem**: Jesus modules are tightly coupled to engine configuration
**Solution**:
- Make modules configurable through initialization parameters
- Use dependency injection for external resources
- Provide sensible defaults for standalone usage

### Challenge 4: Request Context and Logging
**Problem**: Jesus modules rely on request-scoped logging and context
**Solution**:
- Remove request-specific features from core modules
- Provide optional context/logging interfaces
- Make logging configurable and optional

## Implementation Strategy

### Step 1: Module Extraction
1. Create new module directories in `go-go-goja/modules/`
2. Extract core functionality from Jesus engine files
3. Remove Jesus-specific dependencies and features
4. Implement `NativeModule` interface
5. Add comprehensive tests

### Step 2: Jesus Integration
1. Import new modules in Jesus engine
2. Replace direct implementations with module usage
3. Add Jesus-specific wrappers where needed
4. Maintain backward compatibility

### Step 3: Documentation and Examples
1. Update go-go-goja README with new modules
2. Create usage examples for each module
3. Document configuration options
4. Provide migration guide for existing Jesus users

## File Structure After Migration

```
go-go-goja/
├── modules/
│   ├── common.go                    # Existing registry system
│   ├── console/
│   │   └── console.go              # Multi-level logging
│   ├── json/
│   │   └── json.go                 # JSON stringify/parse
│   ├── http-client/
│   │   └── http-client.go          # Fetch API and HTTP methods
│   ├── database/
│   │   ├── database.go             # Core database operations
│   │   └── drivers.go              # Driver registration system
│   ├── express/
│   │   ├── express.go              # Express-like server
│   │   ├── request.go              # Request object
│   │   ├── response.go             # Response object
│   │   └── router.go               # Routing logic
│   ├── exec/                       # Existing
│   └── fs/                         # Existing
└── engine/
    └── runtime.go                  # Updated with new module imports
```

## Benefits of Migration

### For go-go-goja Users
- Rich set of JavaScript APIs out of the box
- Consistent API design across modules
- Well-tested and production-ready implementations
- Easy integration with existing goja applications

### For Jesus
- Cleaner separation of concerns
- Reduced coupling between components
- Easier testing and maintenance
- Reusable components across projects

### For the Ecosystem
- Standardized JavaScript APIs for goja applications
- Community contributions and improvements
- Better documentation and examples
- Wider adoption of goja for JavaScript embedding

## Risks and Mitigation

### Risk 1: Breaking Changes
**Mitigation**: Maintain backward compatibility in Jesus, provide migration guides

### Risk 2: Performance Impact
**Mitigation**: Benchmark before and after migration, optimize module loading

### Risk 3: Increased Complexity
**Mitigation**: Start with simple modules, iterate based on feedback

### Risk 4: Dependency Conflicts
**Mitigation**: Careful dependency management, optional dependencies where possible

## Timeline Estimate

- **Phase 1** (Console, JSON): 1-2 weeks
- **Phase 2** (HTTP Client): 2-3 weeks  
- **Phase 3** (Database): 3-4 weeks
- **Phase 4** (Express Server): 4-5 weeks
- **Documentation and Testing**: 2-3 weeks

**Total Estimated Time**: 12-17 weeks

## Success Criteria

1. All Jesus functionality preserved after migration
2. New modules work independently in go-go-goja
3. Performance impact < 5% for Jesus use cases
4. Comprehensive test coverage (>80%) for new modules
5. Complete documentation and examples
6. Zero breaking changes for existing Jesus users

## Next Steps

1. **Validate Approach**: Review this plan with stakeholders
2. **Start with Phase 1**: Begin with console and JSON modules
3. **Create Proof of Concept**: Implement one module end-to-end
4. **Iterate and Refine**: Adjust approach based on initial results
5. **Scale Implementation**: Apply lessons learned to remaining modules

## Conclusion

Moving JavaScript libraries from Jesus to go-go-goja represents a significant architectural improvement that will benefit both projects and the broader goja ecosystem. The modular approach outlined here minimizes risk while maximizing reusability and maintainability.

The extraction will transform go-go-goja from a simple module playground into a comprehensive JavaScript runtime environment, while making Jesus more focused and maintainable. 