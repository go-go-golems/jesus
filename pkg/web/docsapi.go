package web

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	gogogojamodules "github.com/go-go-golems/go-go-goja/modules"
	"github.com/go-go-golems/jesus/pkg/doc"
)

// CodeExample represents a JavaScript code example extracted from docs
type CodeExample struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Code        string `json:"code"`
	Source      string `json:"source"`   // Which file it came from
	Category    string `json:"category"` // Type of example
}

// DocsAPIHandler handles requests for documentation and code examples
func DocsAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")

		switch action {
		case "examples":
			handleExamples(w, r)
		case "list":
			handleDocsList(w, r)
		case "content":
			handleDocContent(w, r)
		case "modules":
			handleModuleDocs(w, r)
		default:
			http.Error(w, "Invalid action. Use: examples, list, content, or modules", http.StatusBadRequest)
		}
	}
}

// handleExamples extracts and returns JavaScript code examples from docs
func handleExamples(w http.ResponseWriter, r *http.Request) {
	docsFS, err := doc.GetJesusDocsFS()
	if err != nil {
		http.Error(w, "Failed to access docs filesystem", http.StatusInternalServerError)
		return
	}

	examples, err := extractCodeExamples(docsFS)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract examples: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(examples); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleDocsList returns a list of available documentation files
func handleDocsList(w http.ResponseWriter, r *http.Request) {
	type docEntry struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}

	docsFS, err := doc.GetJesusDocsFS()
	if err != nil {
		http.Error(w, "Failed to access docs filesystem", http.StatusInternalServerError)
		return
	}

	files, err := fs.ReadDir(docsFS, ".")
	if err != nil {
		http.Error(w, "Failed to read docs directory", http.StatusInternalServerError)
		return
	}

	var docs []docEntry
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		name := file.Name()
		docs = append(docs, docEntry{
			Name:  name,
			Title: humanizeDocTitle(name),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(docs); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleDocContent returns the content of a specific documentation file
func handleDocContent(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Missing file parameter", http.StatusBadRequest)
		return
	}

	cleanPath := filepath.Clean(filename)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}
	if filepath.Ext(cleanPath) != ".md" {
		http.Error(w, "Only markdown docs are supported", http.StatusBadRequest)
		return
	}

	docsFS, err := doc.GetJesusDocsFS()
	if err != nil {
		http.Error(w, "Failed to access docs filesystem", http.StatusInternalServerError)
		return
	}

	content, err := fs.ReadFile(docsFS, cleanPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	response := map[string]string{
		"filename": cleanPath,
		"content":  string(content),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleModuleDocs returns documentation for registered go-go-goja native modules
func handleModuleDocs(w http.ResponseWriter, r *http.Request) {
	type moduleEntry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	docs := gogogojamodules.DefaultRegistry.GetDocumentation()
	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]moduleEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, moduleEntry{
			Name:        name,
			Description: strings.TrimSpace(docs[name]),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// extractCodeExamples extracts JavaScript code blocks from markdown files
func extractCodeExamples(docsFS fs.FS) ([]CodeExample, error) {
	var examples []CodeExample

	// Regular expression to match JavaScript code blocks
	jsCodeBlockRe := regexp.MustCompile("(?s)" + "```(?:javascript|js)\\n(.*?)\\n```")

	files, err := fs.ReadDir(docsFS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read docs directory: %v", err)
	}

	exampleID := 1
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		content, err := fs.ReadFile(docsFS, file.Name())
		if err != nil {
			continue
		}

		fileContent := string(content)
		category := getCategoryFromFilename(file.Name())

		// Find all JavaScript code blocks
		matches := jsCodeBlockRe.FindAllStringSubmatch(fileContent, -1)

		for _, match := range matches {
			if len(match) > 1 {
				code := strings.TrimSpace(match[1])
				if len(code) == 0 {
					continue
				}

				// Generate a name and description based on the code content
				name, description := generateExampleMetadata(code, category)

				example := CodeExample{
					ID:          fmt.Sprintf("doc-example-%d", exampleID),
					Name:        name,
					Description: description,
					Code:        code,
					Source:      file.Name(),
					Category:    category,
				}

				examples = append(examples, example)
				exampleID++
			}
		}
	}

	return examples, nil
}

// getCategoryFromFilename determines the category based on the filename
func getCategoryFromFilename(filename string) string {
	switch {
	case strings.Contains(filename, "javascript-developer"):
		return "Developer Guide"
	case strings.Contains(filename, "server-architecture"):
		return "Architecture"
	case strings.Contains(filename, "repository"):
		return "Repository Pattern"
	case strings.Contains(filename, "templ"):
		return "Template Integration"
	default:
		return "General"
	}
}

// generateExampleMetadata creates a name and description for a code example
func generateExampleMetadata(code string, category string) (string, string) {
	lines := strings.Split(code, "\n")

	// Look for comments that might indicate what the example does
	var firstComment string
	var isRouteExample bool
	var isDbExample bool
	var isMiddlewareExample bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for route definitions
		if strings.Contains(trimmed, "app.get") || strings.Contains(trimmed, "app.post") ||
			strings.Contains(trimmed, "app.put") || strings.Contains(trimmed, "app.delete") {
			isRouteExample = true
		}

		// Check for database operations
		if strings.Contains(trimmed, "db.query") || strings.Contains(trimmed, "db.execute") ||
			strings.Contains(trimmed, "db.get") || strings.Contains(trimmed, "db.all") {
			isDbExample = true
		}

		// Check for middleware
		if strings.Contains(trimmed, "app.use") || strings.Contains(trimmed, "next()") {
			isMiddlewareExample = true
		}

		// Capture first comment as potential description
		if strings.HasPrefix(trimmed, "//") && firstComment == "" {
			firstComment = strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
		}
	}

	// Generate name based on code content
	var name string
	var description string

	if isRouteExample {
		name = "API Routes"
		description = "Express.js route handling example"
	} else if isDbExample {
		name = "Database Operations"
		description = "SQLite database interaction example"
	} else if isMiddlewareExample {
		name = "Middleware"
		description = "Express.js middleware example"
	} else {
		name = "JavaScript Example"
		description = "General JavaScript code example"
	}

	// Use first comment as description if available and more descriptive
	if firstComment != "" && len(firstComment) > 10 {
		description = firstComment
	}

	// Add category context
	if category != "General" {
		name = fmt.Sprintf("%s - %s", category, name)
	}

	return name, description
}

func humanizeDocTitle(filename string) string {
	title := strings.TrimSuffix(filename, ".md")
	title = strings.ReplaceAll(title, "-", " ")
	title = strings.TrimSpace(title)

	parts := strings.Fields(title)
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}

		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}

	if len(parts) == 0 {
		return title
	}

	return strings.Join(parts, " ")
}
