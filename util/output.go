package util

import (
	"fmt"
)

// PrintProcess prints process information with the "> " prefix
func PrintProcess(format string, args ...interface{}) {
	if len(args) == 0 {
		fmt.Printf("> %s\n", format)
	} else {
		fmt.Printf("> "+format, args...)
	}
}

// PrintSuccess prints success information with the "[√] " prefix
func PrintSuccess(format string, args ...interface{}) {
	if len(args) == 0 {
		fmt.Printf("[√] %s\n", format)
	} else {
		fmt.Printf("[√] "+format, args...)
	}
}

// PrintError prints error information with the "[×] " prefix
func PrintError(format string, args ...interface{}) {
	if len(args) == 0 {
		fmt.Printf("[×] %s\n", format)
	} else {
		fmt.Printf("[×] "+format, args...)
	}
}

// PrintWarning prints warning information with the "[!] " prefix
func PrintWarning(format string, args ...interface{}) {
	if len(args) == 0 {
		fmt.Printf("[!] %s\n", format)
	} else {
		fmt.Printf("[!] "+format, args...)
	}
}
