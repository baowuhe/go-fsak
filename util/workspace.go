package util

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetWorkspaceDir returns the path to the workspace directory
// It checks the FSAK_WS_DIR environment variable first, then defaults to:
// - $HOME/.local/share/fsak on Linux/Mac
// - %LOCALAPPDATA%\fsak on Windows
func GetWorkspaceDir() (string, error) {
	// Check if FSAK_WS_DIR environment variable is set
	wsDir := os.Getenv("FSAK_WS_DIR")
	if wsDir == "" {
		// Use default location based on OS
		if runtime.GOOS == "windows" {
			// On Windows, use %LOCALAPPDATA%\fsak
			localAppData := os.Getenv("LOCALAPPDATA")
			if localAppData == "" {
				// Fallback to USERPROFILE if LOCALAPPDATA is not set
				userProfile := os.Getenv("USERPROFILE")
				if userProfile == "" {
					homeDir, err := os.UserHomeDir()
					if err != nil {
						return "", err
					}
					wsDir = filepath.Join(homeDir, "AppData", "Local", "fsak")
				} else {
					wsDir = filepath.Join(userProfile, "AppData", "Local", "fsak")
				}
			} else {
				wsDir = filepath.Join(localAppData, "fsak")
			}
		} else {
			// On Linux/Mac, use $HOME/.local/share/fsak
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			wsDir = filepath.Join(homeDir, ".local", "share", "fsak")
		}
	} else {
		// Ensure FSAK_WS_DIR is treated as an absolute path
		if !filepath.IsAbs(wsDir) {
			absPath, err := filepath.Abs(wsDir)
			if err != nil {
				return "", err
			}
			wsDir = absPath
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		return "", err
	}

	return wsDir, nil
}

// GetDBPath returns the path to the database file
func GetDBPath() (string, error) {
	wsDir, err := GetWorkspaceDir()
	if err != nil {
		return "", err
	}
	dbDir := filepath.Join(wsDir, "db")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dbDir, "fsak.db"), nil
}
