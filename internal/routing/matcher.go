package routing

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	paramRegex     = regexp.MustCompile(`{([a-zA-Z0-9_]+)(?::([^}]+))?}`)
	extractParamRe = regexp.MustCompile(`{([a-zA-Z0-9_]+)(?::[^}]+)?}`)
)

// PatternMatcher handles matching a URL path against a pattern and extracting parameters.
type PatternMatcher struct {
	Pattern    string
	Regex      *regexp.Regexp
	ParamNames []string
}

// Match checks if the given path matches the pattern and returns the extracted parameters.
func (m *PatternMatcher) Match(path string) (map[string]string, bool) {
	matches := m.Regex.FindStringSubmatch(path)
	if matches == nil {
		return nil, false
	}

	params := make(map[string]string)
	for i, name := range m.Regex.SubexpNames() {
		if i != 0 && name != "" {
			params[name] = matches[i]
		}
	}

	return params, true
}

// CompilePattern converts a pattern string (e.g., "/users/{id}") into a PatternMatcher.
func CompilePattern(pattern string) (*PatternMatcher, error) {
	paramNames := extractParamNames(pattern)
	seen := make(map[string]bool)
	for _, name := range paramNames {
		if seen[name] {
			return nil, fmt.Errorf("duplicate parameter name: %s", name)
		}
		seen[name] = true
	}

	// We need to parse the pattern to identify {name} and {name:regex} parts,
	// escape the literal parts, and convert the parameter parts into named capture groups.
	var res strings.Builder
	re := paramRegex

	lastIndex := 0
	matches := re.FindAllStringSubmatchIndex(pattern, -1)

	for _, m := range matches {
		// Literal part before the match
		literal := pattern[lastIndex:m[0]]
		res.WriteString(regexp.QuoteMeta(literal))

		name := pattern[m[2]:m[3]]
		customRegex := ""
		if m[4] != -1 && m[5] != -1 {
			customRegex = pattern[m[4]:m[5]]
		}

		if customRegex == "" {
			customRegex = "[^/]+"
		}

		fmt.Fprintf(&res, "(?P<%s>%s)", name, customRegex)
		lastIndex = m[1]
	}

	// Final literal part
	res.WriteString(regexp.QuoteMeta(pattern[lastIndex:]))

	combined := res.String()
	// Wildcard * (QuoteMeta escaped it to \*)
	combined = strings.ReplaceAll(combined, `\*`, `.*`)

	compiled, err := regexp.Compile("^" + combined + "$")
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %w", err)
	}

	return &PatternMatcher{
		Pattern:    pattern,
		Regex:      compiled,
		ParamNames: paramNames,
	}, nil
}

func extractParamNames(pattern string) []string {
	re := extractParamRe
	matches := re.FindAllStringSubmatch(pattern, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	return names
}

// GetLiteralPrefix returns the literal part of the pattern before the first parameter or wildcard.
func GetLiteralPrefix(pattern string) string {
	idx := strings.IndexAny(pattern, "{*")
	if idx == -1 {
		return pattern
	}
	return pattern[:idx]
}
