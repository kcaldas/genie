package testing

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertionHelper provides enhanced assertion capabilities with retry logic
type AssertionHelper struct {
	t *testing.T
}

// NewAssertionHelper creates a new assertion helper
func NewAssertionHelper(t *testing.T) *AssertionHelper {
	return &AssertionHelper{t: t}
}

// TextMatcher provides flexible text matching patterns
type TextMatcher struct {
	matchType string
	expected  string
	context   string
}

// Contains creates a matcher that checks if text contains the expected string
func Contains(expected string) *TextMatcher {
	return &TextMatcher{
		matchType: "contains",
		expected:  expected,
	}
}

// Equals creates a matcher that checks for exact text equality
func Equals(expected string) *TextMatcher {
	return &TextMatcher{
		matchType: "equals",
		expected:  expected,
	}
}

// DoesNotContain creates a matcher that checks if text does not contain the expected string
func DoesNotContain(expected string) *TextMatcher {
	return &TextMatcher{
		matchType: "doesNotContain",
		expected:  expected,
	}
}

// IsEmpty creates a matcher that checks if text is empty
func IsEmpty() *TextMatcher {
	return &TextMatcher{
		matchType: "isEmpty",
		expected:  "",
	}
}

// IsNotEmpty creates a matcher that checks if text is not empty
func IsNotEmpty() *TextMatcher {
	return &TextMatcher{
		matchType: "isNotEmpty",
		expected:  "",
	}
}

// WithContext adds context to the matcher for better error messages
func (m *TextMatcher) WithContext(context string) *TextMatcher {
	m.context = context
	return m
}

// Test evaluates the matcher against the actual value
func (m *TextMatcher) Test(actual string) (bool, string) {
	switch m.matchType {
	case "contains":
		matches := strings.Contains(actual, m.expected)
		if !matches {
			return false, fmt.Sprintf("Expected text to contain '%s', but got: '%s'", m.expected, actual)
		}
		return true, ""

	case "equals":
		matches := actual == m.expected
		if !matches {
			return false, fmt.Sprintf("Expected text to equal '%s', but got: '%s'", m.expected, actual)
		}
		return true, ""

	case "doesNotContain":
		matches := !strings.Contains(actual, m.expected)
		if !matches {
			return false, fmt.Sprintf("Expected text to not contain '%s', but got: '%s'", m.expected, actual)
		}
		return true, ""

	case "isEmpty":
		matches := actual == ""
		if !matches {
			return false, fmt.Sprintf("Expected text to be empty, but got: '%s'", actual)
		}
		return true, ""

	case "isNotEmpty":
		matches := actual != ""
		if !matches {
			return false, "Expected text to not be empty, but it was empty"
		}
		return true, ""

	default:
		return false, fmt.Sprintf("Unknown matcher type: %s", m.matchType)
	}
}

// Eventually waits for a condition to become true with retry logic
func (h *AssertionHelper) Eventually(condition func() bool, timeout time.Duration, msgAndArgs ...interface{}) {
	h.t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)

	for {
		if condition() {
			return
		}

		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				if len(msgAndArgs) > 0 {
					require.Fail(h.t, "Condition not met within timeout", msgAndArgs...)
				} else {
					require.Fail(h.t, "Condition not met within timeout")
				}
				return
			}
		}
	}
}

// EventuallyWithError waits for a condition to become true and returns an error message if it fails
func (h *AssertionHelper) EventuallyWithError(condition func() (bool, string), timeout time.Duration, msgAndArgs ...interface{}) {
	h.t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)
	var lastError string

	for {
		success, errorMsg := condition()
		if success {
			return
		}
		lastError = errorMsg

		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				errorMsg := fmt.Sprintf("Condition not met within timeout. Last error: %s", lastError)
				if len(msgAndArgs) > 0 {
					require.Fail(h.t, errorMsg, msgAndArgs...)
				} else {
					require.Fail(h.t, errorMsg)
				}
				return
			}
		}
	}
}

// MatchText waits for text to match a given matcher with retry logic
func (h *AssertionHelper) MatchText(getValue func() string, matcher *TextMatcher, timeout time.Duration) {
	h.t.Helper()

	context := matcher.context
	if context == "" {
		context = "text content"
	}

	h.EventuallyWithError(func() (bool, string) {
		value := getValue()
		return matcher.Test(value)
	}, timeout, "Text match failed for %s", context)
}

// AssertText immediately asserts text matches a given matcher
func (h *AssertionHelper) AssertText(actual string, matcher *TextMatcher) {
	h.t.Helper()

	success, errorMsg := matcher.Test(actual)
	if !success {
		context := matcher.context
		if context != "" {
			errorMsg = fmt.Sprintf("%s: %s", context, errorMsg)
		}
		require.Fail(h.t, errorMsg)
	}
}

// WaitForCount waits for a count to reach an expected value
func (h *AssertionHelper) WaitForCount(getCount func() int, expected int, timeout time.Duration, itemName string) {
	h.t.Helper()

	h.Eventually(func() bool {
		return getCount() == expected
	}, timeout, "Expected %s count to be %d", itemName, expected)
}

// WaitForTrue waits for a condition to become true
func (h *AssertionHelper) WaitForTrue(condition func() bool, timeout time.Duration, description string) {
	h.t.Helper()

	h.Eventually(condition, timeout, "Expected condition to be true: %s", description)
}

// WaitForFalse waits for a condition to become false
func (h *AssertionHelper) WaitForFalse(condition func() bool, timeout time.Duration, description string) {
	h.t.Helper()

	h.Eventually(func() bool {
		return !condition()
	}, timeout, "Expected condition to be false: %s", description)
}

// AssertEventuallyEqual waits for a value to equal expected with retry logic
func (h *AssertionHelper) AssertEventuallyEqual(getValue func() interface{}, expected interface{}, timeout time.Duration, msgAndArgs ...interface{}) {
	h.t.Helper()

	h.Eventually(func() bool {
		actual := getValue()
		return assert.ObjectsAreEqual(expected, actual)
	}, timeout, msgAndArgs...)

	// Final assertion with detailed error message
	actual := getValue()
	assert.Equal(h.t, expected, actual, msgAndArgs...)
}

// AssertEventuallyNotEqual waits for a value to not equal expected with retry logic
func (h *AssertionHelper) AssertEventuallyNotEqual(getValue func() interface{}, expected interface{}, timeout time.Duration, msgAndArgs ...interface{}) {
	h.t.Helper()

	h.Eventually(func() bool {
		actual := getValue()
		return !assert.ObjectsAreEqual(expected, actual)
	}, timeout, msgAndArgs...)

	// Final assertion with detailed error message
	actual := getValue()
	assert.NotEqual(h.t, expected, actual, msgAndArgs...)
}

// Scenario represents a single test scenario for table-driven tests
type Scenario struct {
	Name string
	Test func(t *testing.T)
}

// RunScenarios runs a set of scenarios as subtests
func RunScenarios(t *testing.T, scenarios []Scenario) {
	for _, scenario := range scenarios {
		t.Run(scenario.Name, scenario.Test)
	}
}