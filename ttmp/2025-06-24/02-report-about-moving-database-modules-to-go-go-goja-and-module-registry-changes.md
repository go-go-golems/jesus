# Technical Report: Database Module Migration & Registry Refactoring

## 1. Executive Summary

This report details the successful migration of the database functionality from the `jesus` project into a reusable, self-contained module within `go-go-goja`. It also covers the simultaneous refactoring of the `go-go-goja` module system into a more robust and extensible registry.

These changes are the first major step in the broader initiative to modularize the `jesus` JavaScript environment, promoting code reuse, better organization, and improved documentation.

## 2. Key Accomplishments

1.  **Database Module Extracted**: The tightly-coupled database logic in `jesus` has been moved to a new, independent `database` module in `go-go-goja`.
2.  **Module Registry Refactored**: The `go-go-goja` module system is now a proper `Registry` struct, providing better organization and new features like documentation support.
3.  **Jesus Integrated with New Module**: The `jesus` engine now uses the new `database` module from `go-go-goja`, cleaning up its internal codebase significantly.
4.  **Local Development Dependency Fixed**: Cross-module development is now correctly handled using a `replace` directive in `jesus/go.mod`.

## 3. Detailed Changes

### 3.1. New `database` Module in `go-go-goja`

A new reusable module for database access was created at `go-go-goja/modules/database/database.go`.

**Key Features**:

*   **Self-Contained**: The module encapsulates all logic for database interaction.
*   **Configurable**: It exposes a `Configure(driver, dsn)` function, allowing consumers to set up any `database/sql`-compatible database.
*   **Public API**: The `DBModule` struct now has public methods (`Configure`, `Query`, `Exec`, `Close`) that can be used from Go code for pre-configuration or direct access.
*   **JS Interface**: It exposes `configure`, `query`, `exec`, and `close` functions to the JavaScript runtime via `require('database')`.

```go
// go-go-goja/modules/database/database.go
package databasemod

import (
    "database/sql"
    // ...
)

type DBModule struct {
	db *sql.DB
}

func (m *DBModule) Name() string { return "database" }

func (m *DBModule) Doc() string {
	return `
Database module provides a simple SQL interface.
...
`
}

func (m *DBModule) Configure(driverName, dataSourceName string) error {
    // ...
}

func (m *DBModule) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
    // ...
}
// ...
```

### 3.2. Refactoring of the `go-go-goja` Module Registry

The module system in `go-go-goja/modules/common.go` was significantly improved.

**Before**: A simple package-level slice of modules.

```go
// Old implementation
var all []NativeModule
func Register(m NativeModule) { all = append(all, m) }
func EnableAll(reg *require.Registry) { /* loop over all */ }
```

**After**: A robust `Registry` struct.

```go
// go-go-goja/modules/common.go - New implementation
type NativeModule interface {
	Name() string
	Doc() string // New method for documentation
	Loader(*goja.Runtime, *goja.Object)
}

type Registry struct {
	modules []NativeModule
}

func (r *Registry) Register(m NativeModule) { /* ... */ }
func (r *Registry) GetModule(name string) NativeModule { /* ... */ }
func (r *Registry) GetDocumentation() map[string]string { /* ... */ }
func (r *Registry) Enable(gojaRegistry *require.Registry) { /* ... */ }

// A default registry is provided for backward compatibility
var DefaultRegistry = NewRegistry()
func Register(m NativeModule) { DefaultRegistry.Register(m) }
```

**Key Improvements**:

*   **Struct-based Registry**: The registry is now a `Registry` struct, allowing for multiple, isolated sets of modules if needed.
*   **Documentation Support**: The `NativeModule` interface now includes a `Doc()` method, and the registry can return all module documentation. All existing modules (`database`, `fs`, `exec`) were updated to provide documentation.
*   **Backward Compatibility**: A `DefaultRegistry` instance and package-level wrapper functions (`Register`, `EnableAll`, `GetModule`) were retained to ensure that existing code continues to work without modification.

### 3.3. Integration into `jesus`

The `jesus` engine was updated to consume these new components.

**Engine Refactoring (`jesus/pkg/engine/engine.go`)**:

*   The engine no longer creates its own `*sql.DB` connection.
*   It now initializes the `go-go-goja` module system.
*   It retrieves the `database` module from the registry and calls `dbModule.Configure()` with the application's database path.
*   For backward compatibility, it injects a global `db` variable into the JavaScript runtime: `const db = require('database');`.

**Bindings Cleanup (`jesus/pkg/engine/bindings.go`)**:

*   The `db` binding was removed from `setupBindings()`.
*   The now-redundant `jsQuery` and `jsExec` functions were deleted, resulting in a much cleaner file.

**Dependency Resolution (`jesus/go.mod`)**:

*   To solve a dependency issue where `go mod tidy` would fetch an old version from GitHub, a `replace` directive was added to `jesus/go.mod`. This forces Go to use the local `go-go-goja` directory, enabling seamless cross-module development.

```
replace github.com/go-go-golems/go-go-goja => ../go-go-goja
```

## 4. Benefits

*   **Modularity & Reusability**: The database logic is no longer a bespoke part of `jesus`. It's now a standalone component in `go-go-goja` that can be used in any project.
*   **Improved Code Quality**: The `jesus` engine's responsibilities are now more focused. The `bindings.go` file is significantly cleaner.
*   **Enhanced Documentation**: The new registry system provides a built-in mechanism for module documentation, improving developer experience.
*   **Clearer Dependencies**: The relationship between `jesus` and `go-go-goja` is now explicit and correctly managed.

## 5. How to Use the Module Registry in an Application (e.g., Jesus REPL)

This section provides a practical guide for integrating the `go-go-goja` module registry into a Go application, such as the `jesus` REPL, to provide a rich, documented JavaScript environment.

### Step 1: Accessing the Registry and Modules

You can access the `DefaultRegistry` to interact with all globally registered modules. This allows you to list modules, get their documentation, and even configure them from your Go code before the JavaScript VM is started.

```go
package main

import (
	"fmt"
	"log"

	gogogojamodules "github.com/go-go-golems/go-go-goja/modules"
	databasemod "github.com/go-go-golems/go-go-goja/modules/database"
	// Other module imports to trigger their init()
	_ "github.com/go-go-golems/go-go-goja/modules/exec"
	_ "github.com/go-go-golems/go-go-goja/modules/fs"
)

func main() {
	// Get the default registry
	registry := gogogojamodules.DefaultRegistry

	// 1. List all registered modules and their documentation
	fmt.Println("Available Modules:")
	for name, doc := range registry.GetDocumentation() {
		fmt.Printf(" - %s: %s\n", name, doc)
	}

	// 2. Get a specific module for pre-configuration
	dbModule, ok := registry.GetModule("database").(*databasemod.DBModule)
	if !ok || dbModule == nil {
		log.Fatal("Database module not found!")
	}

	// 3. Configure the module from Go
	err := dbModule.Configure("sqlite3", "/tmp/my-app.db")
	if err != nil {
		log.Fatalf("Failed to configure database: %v", err)
	}
	fmt.Println("\nDatabase module configured from Go.")

	// The VM can now be started, and JS code can use `require('database')`
	// without needing to call `configure()` again.
}
```

### Step 2: Integrating with a REPL

In an interactive REPL, you can use the registry to provide helpful features to the user, like a `:modules` command to list all available native modules.

```go
// Inside a REPL command handler
case ":modules":
    docs := gogogojamodules.DefaultRegistry.GetDocumentation()
    fmt.Println("Available native modules:")
    for name := range docs {
        fmt.Printf("  - %s\n", name)
    }
    // For more detail:
    // fmt.Println(docs[moduleName])

// ...
```

This integration makes the JavaScript environment more transparent and easier to use for developers by exposing the underlying Go-based functionality in a structured and documented way.

## 6. Next Steps

With this foundational work complete, we are well-positioned to continue migrating other core functionalities (HTTP client, Express server) from `jesus` into `go-go-goja` modules, following the same successful pattern. 