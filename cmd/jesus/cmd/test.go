package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

// TestCmd represents the test command
type TestCmd struct {
	*cmds.CommandDescription
}

// TestSettings holds the configuration for the test command
type TestSettings struct {
	URL      string `glazed:"url"`
	AdminURL string `glazed:"admin-url"`
}

// Ensure TestCmd implements BareCommand
var _ cmds.BareCommand = &TestCmd{}

// NewTestCmd creates a new test command
func NewTestCmd() (*TestCmd, error) {
	return &TestCmd{
		CommandDescription: cmds.NewCommandDescription(
			"test",
			cmds.WithShort("Test the server endpoints"),
			cmds.WithLong(`
Test the JavaScript playground server endpoints to verify functionality.

This command performs a series of tests:
1. Health endpoint (/health) - Basic server availability
2. Root endpoint (/) - Main server response
3. Counter endpoint (/counter) - State management test
4. Execute endpoint (/v1/execute) - JavaScript execution test
5. Dynamic endpoint test - Verify runtime route creation

The tests validate:
- Server connectivity and responsiveness
- JavaScript engine functionality
- Database integration
- Dynamic route registration

Examples:
  test
  test --url http://localhost:9923
			`),
			cmds.WithFlags(
				fields.New(
					"url",
					fields.TypeString,
					fields.WithHelp("Main server URL to test"),
					fields.WithDefault("http://localhost:9922"),
					fields.WithShortFlag("u"),
				),
				fields.New(
					"admin-url",
					fields.TypeString,
					fields.WithHelp("Admin server URL for execute endpoint testing"),
					fields.WithDefault("http://localhost:9090"),
					fields.WithShortFlag("a"),
				),
			),
		),
	}, nil
}

// Run implements the BareCommand interface
func (c *TestCmd) Run(ctx context.Context, parsedValues *values.Values) error {
	// Parse settings from the default section.
	s := &TestSettings{}
	if err := parsedValues.DecodeSectionInto(schema.DefaultSlug, s); err != nil {
		return errors.Wrap(err, "failed to parse test settings")
	}

	log.Info().Str("url", s.URL).Msg("Testing server")

	var testResults []TestResult

	// Test 1: Health endpoint
	log.Info().Msg("Testing health endpoint")
	result := c.testEndpoint("GET", s.URL+"/health", "", "Health endpoint")
	testResults = append(testResults, result)
	c.logTestResult(result)

	// Test 2: Root endpoint
	log.Info().Msg("Testing root endpoint")
	result = c.testEndpoint("GET", s.URL+"/", "", "Root endpoint")
	testResults = append(testResults, result)
	c.logTestResult(result)

	// Test 3: Counter endpoint
	log.Info().Msg("Testing counter endpoint")
	result = c.testEndpoint("POST", s.URL+"/counter", "{}", "Counter endpoint")
	testResults = append(testResults, result)
	c.logTestResult(result)

	// Test 4: Execute endpoint by registering a dynamic route on the admin port.
	log.Info().Msg("Testing execute endpoint")
	testCode := `
		console.log("Registering test route");

		// Register a test endpoint
		registerHandler("GET", "/test", () => ({
			message: "Test endpoint works!", 
			time: new Date().toISOString()
		}));
		
		"Execute test completed"
	`

	result = c.testExecuteEndpoint(s.AdminURL+"/v1/execute", testCode, "Execute endpoint")
	testResults = append(testResults, result)
	c.logTestResult(result)

	// Test 5: Newly created dynamic endpoint
	log.Info().Msg("Testing dynamically created endpoint")
	result = c.testEndpoint("GET", s.URL+"/test", "", "Dynamic endpoint")
	testResults = append(testResults, result)
	c.logTestResult(result)

	// Summary
	c.printTestSummary(testResults)

	// Check if any tests failed
	for _, result := range testResults {
		if !result.Success {
			return fmt.Errorf("test failures detected - see output above")
		}
	}

	return nil
}

// TestResult represents the result of a single test
type TestResult struct {
	Name    string
	Success bool
	Status  string
	Body    string
	Error   error
}

// testEndpoint performs a single endpoint test
func (c *TestCmd) testEndpoint(method, url, body, name string) TestResult {
	var resp *http.Response
	var err error

	switch method {
	case "GET":
		resp, err = http.Get(url)
	case "POST":
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		resp, err = http.Post(url, "application/json", reader)
	default:
		return TestResult{
			Name:    name,
			Success: false,
			Error:   fmt.Errorf("unsupported HTTP method: %s", method),
		}
	}

	if err != nil {
		return TestResult{
			Name:    name,
			Success: false,
			Error:   err,
		}
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(resp.Body)
	bodyStr := ""
	if readErr == nil {
		bodyStr = string(responseBody)
	}

	return TestResult{
		Name:    name,
		Success: resp.StatusCode < 400,
		Status:  resp.Status,
		Body:    bodyStr,
		Error:   readErr,
	}
}

// testExecuteEndpoint performs a specialized test for the execute endpoint with JavaScript content
func (c *TestCmd) testExecuteEndpoint(url, jsCode, name string) TestResult {
	resp, err := http.Post(url, "application/javascript", strings.NewReader(jsCode))
	if err != nil {
		return TestResult{
			Name:    name,
			Success: false,
			Error:   err,
		}
	}
	defer resp.Body.Close()

	responseBody, readErr := io.ReadAll(resp.Body)
	bodyStr := ""
	if readErr == nil {
		bodyStr = string(responseBody)
	}

	return TestResult{
		Name:    name,
		Success: resp.StatusCode < 400,
		Status:  resp.Status,
		Body:    bodyStr,
		Error:   readErr,
	}
}

// logTestResult logs the result of a test
func (c *TestCmd) logTestResult(result TestResult) {
	if result.Success {
		log.Info().
			Str("test", result.Name).
			Str("status", result.Status).
			Str("body", truncateString(result.Body, 200)).
			Msg("✅ Test passed")
	} else {
		log.Error().
			Str("test", result.Name).
			Err(result.Error).
			Msg("❌ Test failed")
	}
}

// printTestSummary prints a summary of all test results
func (c *TestCmd) printTestSummary(results []TestResult) {
	fmt.Println("\n=== Test Summary ===")

	passed := 0
	failed := 0

	for _, result := range results {
		if result.Success {
			fmt.Printf("✅ %s: PASSED (%s)\n", result.Name, result.Status)
			passed++
		} else {
			fmt.Printf("❌ %s: FAILED", result.Name)
			if result.Error != nil {
				fmt.Printf(" - %s", result.Error.Error())
			}
			fmt.Println()
			failed++
		}
	}

	fmt.Printf("\nTotal: %d tests, %d passed, %d failed\n", len(results), passed, failed)

	if failed == 0 {
		fmt.Println("🎉 All tests passed!")
	} else {
		fmt.Printf("⚠️  %d test(s) failed\n", failed)
	}
}

// truncateString truncates a string for display purposes
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
