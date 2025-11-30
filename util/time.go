package util

import (
	"os"
	"syscall"
	"time"
)

// GetCreationTime returns the creation time of a file
// On Unix systems, this returns the change time (ctime) which is the closest to creation time
// On Windows, this would return the actual creation time
func GetCreationTime(info os.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		// On Unix systems, use the change time (ctime)
		return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
	}

	// Fallback to ModTime if we can't get the creation time
	return info.ModTime()
}
