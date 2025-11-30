package util

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// ReadBlacklist reads the blacklist file and returns compiled regex patterns
func ReadBlacklist(blacklistFile string) ([]*regexp.Regexp, error) {
	if blacklistFile == "" {
		return []*regexp.Regexp{}, nil
	}

	file, err := os.Open(blacklistFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []*regexp.Regexp
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // Skip empty lines
		}

		// Check if the line is a regex pattern (starts and ends with /)
		if len(line) >= 2 && line[0] == '/' && line[len(line)-1] == '/' {
			pattern := line[1 : len(line)-1] // Remove the leading and trailing '/'
			regex, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			patterns = append(patterns, regex)
		} else {
			// Treat as a literal path - escape special regex characters
			escapedLine := regexp.QuoteMeta(line)
			regex, err := regexp.Compile("^" + escapedLine + "$")
			if err != nil {
				return nil, err
			}
			patterns = append(patterns, regex)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}
