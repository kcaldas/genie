package slashcommands

import (
	"regexp"
	"strings"
)

// Define the regex pattern for $ARGUMENTS
var argsPattern = regexp.MustCompile(`\$ARGUMENTS`)

func ExpandArguments(input string, originalArgs []string) string {
	// Find all matches of $ARGUMENTS
	matches := argsPattern.FindAllStringIndex(input, -1)
	numPlaceholders := len(matches)

	if numPlaceholders == 0 {
		return input
	}

	var resultBuilder strings.Builder
	lastIndex := 0
	argCounter := 0 // This will track which argument from originalArgs to use for non-last placeholders

	for i, match := range matches {
		// Add the part of the string before the current $ARGUMENTS
		resultBuilder.WriteString(input[lastIndex:match[0]])

		// Check if this is the last $ARGUMENTS placeholder
		if i == numPlaceholders-1 {
			// If it's the last, append all remaining arguments joined by space
			resultBuilder.WriteString(strings.Join(originalArgs[argCounter:], " "))
		} else {
			// If not the last, append a single argument if available
			if argCounter < len(originalArgs) {
				resultBuilder.WriteString(originalArgs[argCounter])
				argCounter++
			} else {
				// If no more arguments, append an empty string
				resultBuilder.WriteString("")
			}
		}
		lastIndex = match[1] // Update lastIndex to after the current $ARGUMENTS
	}

	// Add the remaining part of the string after the last $ARGUMENTS
	resultBuilder.WriteString(input[lastIndex:])

	return resultBuilder.String()
}
