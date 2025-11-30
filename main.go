package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/baowuhe/go-fsak/core"
	"github.com/baowuhe/go-fsak/util"
)

func main() {
	// Print workspace directory
	wsDir, err := util.GetWorkspaceDir()
	if err != nil {
		util.PrintError("Error getting workspace directory: %v\n", err)
		os.Exit(1)
	}

	// Get current user to show more user-friendly path
	currentUser, err := os.UserHomeDir()
	if err == nil {
		// Replace home directory path with ~ for brevity, but only if the workspace directory is under the home directory
		relPath, err2 := filepath.Rel(currentUser, wsDir)
		if err2 == nil && relPath != ".." && !strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			wsDir = filepath.Join("~", relPath)
		}
	}

	util.PrintProcess("Workspace directory: %s\n", wsDir)

	if err := core.Execute(); err != nil {
		util.PrintError("%v", err)
		os.Exit(1)
	}
}
