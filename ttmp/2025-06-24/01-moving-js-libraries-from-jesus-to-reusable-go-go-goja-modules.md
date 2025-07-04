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

## goja_nodejs: The Missing Piece

**CRITICAL DISCOVERY**: There's already a comprehensive Node.js compatibility library at `goja_nodejs/` that provides many of the modules we were planning to extract from Jesus!

### Available goja_nodejs Modules
1. **console** - Multi-level logging with configurable printers
2. **buffer** - Complete Node.js Buffer implementation with encoding support
3. **url** - URL parsing and URLSearchParams
4. **util** - String formatting utilities (used by console)
5. **process** - Environment variables and process info
6. **eventloop** - Async operations with setTimeout, setInterval, Promises
7. **require** - Module loading system (core infrastructure)
8. **errors** - Node.js compatible error handling

### Key Insights

#### What This Changes
- **Console Module**: ✅ Already exists in goja_nodejs with better implementation than Jesus
- **JSON Module**: ❌ Not in goja_nodejs, but Jesus's implementation is basic
- **EventLoop/Async**: ✅ Already exists with full Promise support and timers
- **Buffer Operations**: ✅ Already exists with comprehensive encoding support
- **URL Handling**: ✅ Already exists with full URL parsing
- **Utilities**: ✅ Already exists with string formatting

#### What We Still Need to Extract
- **Database Module**: Not available in goja_nodejs
- **HTTP Client Module**: Not available in goja_nodejs  
- **Express Server Module**: Not available in goja_nodejs
- **Global State Module**: Not available in goja_nodejs (Jesus-specific)

## Revised Migration Plan

### Phase 0: Leverage Existing goja_nodejs Modules (IMMEDIATE)

#### 0.1 Replace Jesus Console with goja_nodejs Console
**Action**: Update go-go-goja to use `github.com/dop251/goja_nodejs/console`
**Benefits**: 
- Better implementation with configurable printers
- Standard Node.js compatibility
- Removes need to extract from Jesus

#### 0.2 Add goja_nodejs Integration to go-go-goja
**Action**: Import and enable key goja_nodejs modules in `engine/runtime.go`
**Modules to Enable**:
- `console` - Logging
- `buffer` - Binary data handling  
- `url` - URL parsing
- `util` - Utilities
- `process` - Environment access
- `eventloop` - Async operations

### Phase 1: Extract Jesus-Specific Modules (Medium-High Complexity)

#### 1.1 Database Module
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

#### 1.2 HTTP Client Module
**Target**: `go-go-goja/modules/http-client/http-client.go`

**Features to Extract**:
- Modern `fetch()` API
- HTTP method shortcuts (`GET`, `POST`, etc.)
- Request/response transformation
- Timeout and header support
- JSON body handling
- Async support using goja_nodejs/eventloop

**Dependencies**: `net/http`, `encoding/json`, `github.com/dop251/goja_nodejs/eventloop`

**Interface**:
```javascript
const { fetch, HTTP } = require("http-client");

// Modern fetch API with Promises
const response = await fetch("https://api.example.com/data");
const data = await response.json();

// Method shortcuts
const result = HTTP.get("https://api.example.com/users");
```

**Implementation Considerations**:
- Remove Jesus-specific request logging
- Make HTTP client configurable (timeouts, etc.)
- Use goja_nodejs eventloop for Promise-based async operations
- Support both synchronous and asynchronous patterns

### Phase 2: Express-like Server Module (High Complexity)

#### 2.1 Express Server Module
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

## Revised Timeline Estimate

- **Phase 0** (goja_nodejs Integration): 1-2 weeks
- **Phase 1** (Database + HTTP Client): 4-6 weeks  
- **Phase 2** (Express Server): 4-5 weeks
- **Documentation and Testing**: 2-3 weeks

**Total Estimated Time**: 11-16 weeks (reduced by leveraging existing goja_nodejs modules)

## Success Criteria

1. All Jesus functionality preserved after migration
2. New modules work independently in go-go-goja
3. Performance impact < 5% for Jesus use cases
4. Comprehensive test coverage (>80%) for new modules
5. Complete documentation and examples
6. Zero breaking changes for existing Jesus users

## Next Steps

1. **Validate Approach**: Review this revised plan with stakeholders
2. **Start with Phase 0**: Integrate goja_nodejs modules into go-go-goja
3. **Update Jesus**: Replace Jesus console with goja_nodejs console
4. **Create Proof of Concept**: Implement database module end-to-end
5. **Iterate and Refine**: Adjust approach based on initial results
6. **Scale Implementation**: Apply lessons learned to remaining modules

## Conclusion

The discovery of the comprehensive goja_nodejs library significantly changes our migration strategy. Instead of extracting and reimplementing basic JavaScript APIs from Jesus, we can:

1. **Leverage existing, battle-tested Node.js compatibility modules** from goja_nodejs
2. **Focus our extraction efforts on Jesus-specific, high-value modules** (database, HTTP client, Express server)
3. **Reduce development time by 1-6 weeks** while getting better implementations

This revised approach will transform go-go-goja into a comprehensive JavaScript runtime environment by combining:
- **Existing goja_nodejs modules** for standard Node.js APIs
- **Extracted Jesus modules** for web development and database access
- **Native go-go-goja modules** for system integration

The result will be a powerful, reusable JavaScript runtime that benefits both the go-go-goja ecosystem and makes Jesus more focused and maintainable. 