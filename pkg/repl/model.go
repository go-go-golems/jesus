package repl

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dop251/goja"
	ggjengine "github.com/go-go-golems/go-go-goja/engine"
)

// Model represents the UI state for the REPL
type Model struct {
	styles              Styles
	jsRuntime           *goja.Runtime
	textInput           textinput.Model
	history             []historyEntry
	historyEntries      []string // Store just the input strings for navigation
	currentHistoryIndex int      // Current position in history (-1 means not navigating)
	multilineMode       bool
	multilineText       []string
	width               int
	quitting            bool
}

// historyEntry represents a single entry in the REPL history
type historyEntry struct {
	input  string
	output string
	isErr  bool
}

// NewModel creates a new UI model
func NewModel(startMultiline bool) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter JavaScript or /command"
	ti.Focus()
	ti.Width = 80
	ti.Prompt = "js> "

	// Create a Goja runtime with Node-style require() and native modules enabled
	// using the go-go-goja engine helper. This ensures that users can load
	// modules (e.g. require("database")) from within the REPL.
	vm, _ := ggjengine.New()
	rt := vm

	// Override console.log to write directly to stdout without timestamps to keep REPL output clean.
	consoleObj := rt.NewObject()
	_ = consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		var args []interface{}
		for _, arg := range call.Arguments {
			args = append(args, arg.Export())
		}
		fmt.Println(args...)
		return goja.Undefined()
	})
	_ = rt.Set("console", consoleObj)

	return Model{
		styles:              DefaultStyles(),
		jsRuntime:           rt,
		textInput:           ti,
		history:             []historyEntry{},
		historyEntries:      []string{},
		currentHistoryIndex: -1, // -1 means not navigating history
		multilineMode:       startMultiline,
		multilineText:       []string{},
		width:               80, // Default width
		quitting:            false,
	}
}

// Init initializes the UI model
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles UI events and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update the width for proper wrapping
		m.width = msg.Width
		m.textInput.Width = msg.Width - 10 // Account for prompt and padding

	case tea.KeyMsg:
		// Check for Ctrl+E to open external editor
		if msg.Type == tea.KeyCtrlE {
			if m.multilineMode && len(m.multilineText) > 0 {
				// Open multiline content in external editor
				if editedContent, err := m.openExternalEditor(strings.Join(m.multilineText, "\n")); err == nil {
					// Update multiline content with edited text
					m.multilineText = strings.Split(editedContent, "\n")
					// Remove empty lines at the end
					for len(m.multilineText) > 0 && m.multilineText[len(m.multilineText)-1] == "" {
						m.multilineText = m.multilineText[:len(m.multilineText)-1]
					}
				} else {
					// Add error to history
					m.history = append(m.history, historyEntry{
						input:  "/edit",
						output: fmt.Sprintf("Editor error: %v", err),
						isErr:  true,
					})
				}
			} else if currentInput := m.textInput.Value(); currentInput != "" {
				// Open current single-line input in external editor
				if editedContent, err := m.openExternalEditor(currentInput); err == nil {
					lines := strings.Split(strings.TrimSpace(editedContent), "\n")
					if len(lines) > 1 {
						// Switch to multiline mode if editor returned multiple lines
						m.multilineMode = true
						m.multilineText = lines
						m.textInput.Reset()
					} else {
						// Update single line input
						m.textInput.SetValue(lines[0])
					}
				} else {
					// Add error to history
					m.history = append(m.history, historyEntry{
						input:  "/edit",
						output: fmt.Sprintf("Editor error: %v", err),
						isErr:  true,
					})
				}
			}
			return m, nil
		}

		// Check for Ctrl+J as a substitute for Shift+Enter (which isn't directly supported)
		if msg.Type == tea.KeyCtrlJ {
			// Handle Ctrl+J for multiline input
			if !m.multilineMode {
				m.multilineMode = true
				m.multilineText = []string{m.textInput.Value()}
			} else {
				m.multilineText = append(m.multilineText, m.textInput.Value())
			}
			m.textInput.Reset()
			return m, nil
		}

		//nolint:exhaustive
		switch msg.Type {
		case tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyUp:
			// Navigate backwards through history (most recent first)
			if len(m.historyEntries) > 0 {
				if m.currentHistoryIndex == -1 {
					// Start from the end (most recent)
					m.currentHistoryIndex = len(m.historyEntries) - 1
				} else if m.currentHistoryIndex > 0 {
					m.currentHistoryIndex--
				}
				m.textInput.SetValue(m.historyEntries[m.currentHistoryIndex])
			}
			return m, nil

		case tea.KeyDown:
			// Navigate forwards through history
			if m.currentHistoryIndex != -1 {
				if m.currentHistoryIndex < len(m.historyEntries)-1 {
					m.currentHistoryIndex++
					m.textInput.SetValue(m.historyEntries[m.currentHistoryIndex])
				} else {
					// At the end, return to empty input
					m.currentHistoryIndex = -1
					m.textInput.Reset()
				}
			}
			return m, nil

		case tea.KeyEnter:
			input := m.textInput.Value()

			// If in multiline mode, check if we should execute or continue
			if m.multilineMode {
				if input == "" {
					// Empty line in multiline mode means execute the code
					fullInput := strings.Join(m.multilineText, "\n")
					m = m.processInput(fullInput)
					m.multilineMode = false
					m.multilineText = []string{}
				} else {
					// Add another line to multiline input
					m.multilineText = append(m.multilineText, input)
					m.textInput.Reset()
					return m, nil
				}
			} else {
				// Normal single-line mode
				if input == "" {
					return m, nil
				}
				m = m.processInput(input)
			}

			m.textInput.Reset()
			m.currentHistoryIndex = -1 // Reset history navigation after processing input

			if m.quitting {
				return m, tea.Quit
			}
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View renders the UI
func (m Model) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(m.styles.Title.Render(" JavaScript REPL (js-web-server) "))
	sb.WriteString("\n\n")

	// History with wrapping
	for _, entry := range m.history {
		// Input
		sb.WriteString(m.styles.Prompt.Render("js> "))
		sb.WriteString(m.wrapText(entry.input, m.width-5))
		sb.WriteString("\n")

		// Output
		if entry.isErr {
			sb.WriteString(m.wrapText(m.styles.Error.Render(entry.output), m.width))
		} else {
			sb.WriteString(m.wrapText(m.styles.Result.Render(entry.output), m.width))
		}
		sb.WriteString("\n\n")
	}

	// Multiline input display
	if m.multilineMode {
		sb.WriteString(m.styles.Info.Render("Multiline Mode (press Enter on empty line to execute, Ctrl+E to edit):\n"))
		for _, line := range m.multilineText {
			sb.WriteString(m.styles.Prompt.Render("... "))
			sb.WriteString(m.wrapText(line, m.width-5))
			sb.WriteString("\n")
		}
	}

	// Input field
	sb.WriteString(m.textInput.View())
	sb.WriteString("\n\n")

	// Help text
	helpText := "Type JavaScript code or /help for commands"
	if m.multilineMode {
		helpText = "Multiline mode: Enter empty line to execute, Ctrl+J for more lines, Ctrl+E to edit, ↑/↓ for history"
	} else {
		helpText += " (Ctrl+J for multiline, Ctrl+E to edit, ↑/↓ for history)"
	}

	sb.WriteString(m.styles.HelpText.Render(helpText))
	sb.WriteString("\n")

	if m.quitting {
		sb.WriteString("\n")
		sb.WriteString(m.styles.Info.Render("Exiting..."))
		sb.WriteString("\n")
	}

	return sb.String()
}

// openExternalEditor opens the given content in an external editor and returns the edited content
func (m Model) openExternalEditor(content string) (string, error) {
	// Get editor from environment variable, fallback to nano or vim
	editor := os.Getenv("EDITOR")
	if editor == "" {
		// Try to find a suitable editor
		for _, candidate := range []string{"nano", "vim", "vi"} {
			if _, err := exec.LookPath(candidate); err == nil {
				editor = candidate
				break
			}
		}
		if editor == "" {
			return "", fmt.Errorf("no suitable editor found. Set $EDITOR environment variable")
		}
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "js-repl-*.js")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to remove temporary file %s: %v\n", tmpFile.Name(), err)
		}
	}() // Clean up the temp file

	// Write current content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close temporary file: %v\n", closeErr)
		}
		return "", fmt.Errorf("failed to write to temporary file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Launch the editor
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor exited with error: %w", err)
	}

	// Read the edited content back
	editedFile, err := os.Open(tmpFile.Name())
	if err != nil {
		return "", fmt.Errorf("failed to read edited file: %w", err)
	}
	defer func() {
		if err := editedFile.Close(); err != nil {
			fmt.Printf("Warning: failed to close edited file: %v\n", err)
		}
	}()

	editedBytes, err := io.ReadAll(editedFile)
	if err != nil {
		return "", fmt.Errorf("failed to read edited content: %w", err)
	}

	return string(editedBytes), nil
}

// wrapText wraps text to fit within the given width
func (m Model) wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var sb strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if len(line) <= width {
			sb.WriteString(line)
		} else {
			// Wrap the line
			currentWidth := 0
			words := strings.Fields(line)
			for j, word := range words {
				wordLen := len(word)
				if currentWidth+wordLen > width {
					// Start a new line with proper indentation
					sb.WriteString("\n    ")
					currentWidth = 4 // Account for indentation
				} else if j > 0 {
					sb.WriteString(" ")
					currentWidth++
				}
				sb.WriteString(word)
				currentWidth += wordLen
			}
		}

		// Add newline between original lines, but not after the last one
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// processInput handles user input and updates the model
func (m Model) processInput(input string) Model {
	// Add non-empty, non-duplicate input to history
	if input != "" && (len(m.historyEntries) == 0 || m.historyEntries[len(m.historyEntries)-1] != input) {
		m.historyEntries = append(m.historyEntries, input)
	}

	if strings.HasPrefix(input, "/") {
		// Handle slash commands
		return m.handleSlashCommand(input)
	}

	// Handle JavaScript evaluation
	result, err := m.jsRuntime.RunString(input)
	if err != nil {
		m.history = append(m.history, historyEntry{
			input:  input,
			output: err.Error(),
			isErr:  true,
		})
		return m
	}

	// Convert result to string
	var output string
	if result != nil && !goja.IsUndefined(result) {
		output = result.String()
	} else {
		output = "undefined"
	}

	m.history = append(m.history, historyEntry{
		input:  input,
		output: output,
		isErr:  false,
	})
	return m
}

// handleSlashCommand processes slash commands
func (m Model) handleSlashCommand(input string) Model {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return m
	}

	cmd := strings.TrimPrefix(parts[0], "/")

	switch cmd {
	case "help":
		helpText := `Available commands:
/help      - Show this help
/clear     - Clear the screen
/quit      - Exit the REPL
/multiline - Toggle multiline mode
/edit      - Open current content in external editor (same as Ctrl+E)

Keyboard shortcuts:
Ctrl+J     - Add line in multiline mode
Ctrl+E     - Open external editor
Ctrl+C     - Exit REPL
Up/Down    - Navigate command history`

		m.history = append(m.history, historyEntry{
			input:  input,
			output: helpText,
			isErr:  false,
		})

	case "clear":
		m.history = []historyEntry{}

	case "quit", "exit":
		m.quitting = true

	case "multiline":
		m.multilineMode = !m.multilineMode
		status := "disabled"
		if m.multilineMode {
			status = "enabled"
		}
		m.history = append(m.history, historyEntry{
			input:  input,
			output: fmt.Sprintf("Multiline mode %s", status),
			isErr:  false,
		})

	case "edit":
		// Handle /edit command - same as Ctrl+E
		var content string
		if m.multilineMode && len(m.multilineText) > 0 {
			content = strings.Join(m.multilineText, "\n")
		} else {
			content = m.textInput.Value()
		}

		if content == "" {
			m.history = append(m.history, historyEntry{
				input:  input,
				output: "No content to edit. Type some code first.",
				isErr:  true,
			})
			return m
		}

		if editedContent, err := m.openExternalEditor(content); err == nil {
			lines := strings.Split(strings.TrimSpace(editedContent), "\n")
			if len(lines) > 1 {
				// Switch to multiline mode if editor returned multiple lines
				m.multilineMode = true
				m.multilineText = lines
				m.textInput.Reset()
			} else {
				// Update single line input
				m.multilineMode = false
				m.multilineText = []string{}
				m.textInput.SetValue(lines[0])
			}
			m.history = append(m.history, historyEntry{
				input:  input,
				output: "Content updated from external editor",
				isErr:  false,
			})
		} else {
			m.history = append(m.history, historyEntry{
				input:  input,
				output: fmt.Sprintf("Editor error: %v", err),
				isErr:  true,
			})
		}

	default:
		m.history = append(m.history, historyEntry{
			input:  input,
			output: fmt.Sprintf("Unknown command: %s", cmd),
			isErr:  true,
		})
	}

	return m
}
