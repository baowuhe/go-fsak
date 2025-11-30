package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/baowuhe/go-fsak/data"
	"github.com/baowuhe/go-fsak/util"
	"github.com/spf13/cobra"
)

// mergeCmd represents the merge command
var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge files from source directory to target directory",
	Long:  `Commands for merging files between directories.`,
}

// dirCmd represents the merge dir command
var dirCmd = &cobra.Command{
	Use:   "dir",
	Short: "Merge files from source directory to target directory",
	Long:  `Traverse source and target directories, calculate MD5 and Blake3 values, and copy files that don't exist in target based on these values.`,
	Run: func(cmd *cobra.Command, args []string) {
		sourceDir, _ := cmd.Flags().GetString("from")
		targetDir, _ := cmd.Flags().GetString("to")

		if sourceDir == "" || targetDir == "" {
			util.PrintError("Both source (-f) and target (-t) directories must be specified\n")
			os.Exit(1)
		}

		// Convert to absolute paths
		var err error
		sourceDir, err = filepath.Abs(sourceDir)
		if err != nil {
			util.PrintError("Error getting absolute path for source: %v\n", err)
			os.Exit(1)
		}
		targetDir, err = filepath.Abs(targetDir)
		if err != nil {
			util.PrintError("Error getting absolute path for target: %v\n", err)
			os.Exit(1)
		}

		// Validate directories exist
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			util.PrintError("Source directory does not exist: %s\n", sourceDir)
			os.Exit(1)
		}
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			util.PrintError("Target directory does not exist: %s\n", targetDir)
			os.Exit(1)
		}

		util.PrintProcess("Starting merge operation from %s to %s\n", sourceDir, targetDir)
		err = performMerge(sourceDir, targetDir)
		if err != nil {
			util.PrintError("Error during merge: %v\n", err)
			os.Exit(1)
		}
		util.PrintSuccess("Merge operation completed successfully.\n")
	},
}

// Initialize the commands
func init() {
	// Add flags to dirCmd
	dirCmd.Flags().StringP("from", "f", "", "Source directory to merge from (required)")
	dirCmd.Flags().StringP("to", "t", "", "Target directory to merge to (required)")

	// Mark required flags
	_ = dirCmd.MarkFlagRequired("from")
	_ = dirCmd.MarkFlagRequired("to")

	// Add dirCmd to mergeCmd
	mergeCmd.AddCommand(dirCmd)

	// Add mergeCmd to rootCmd
	rootCmd.AddCommand(mergeCmd)
}

// performMerge executes the merge operation between source and target directories
func performMerge(sourceDir, targetDir string) error {
	// Connect to database
	db, err := data.Connect()
	if err != nil {
		return fmt.Errorf("error connecting to database: %v", err)
	}
	defer func() {
		sqlDB, _ := db.DB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	// Create FSAK_<YYMMdd> directory in target
	dateStr := time.Now().Format("060102") // YYMMdd format
	backupDir := filepath.Join(targetDir, fmt.Sprintf("FSAK_%s", dateStr))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("error creating backup directory: %v", err)
	}
	util.PrintProcess("Created backup directory: %s\n", backupDir)

	// Get all files in source and target directories and their MD5/Blake3 values
	sourceFiles, err := getFilesWithHashes(db, sourceDir)
	if err != nil {
		return fmt.Errorf("error getting source files: %v", err)
	}
	util.PrintProcess("Found %d files in source directory\n", len(sourceFiles))

	targetFiles, err := getFilesWithHashes(db, targetDir)
	if err != nil {
		return fmt.Errorf("error getting target files: %v", err)
	}
	util.PrintProcess("Found %d files in target directory\n", len(targetFiles))

	// Find files from source that don't exist in target based on MD5 and Blake3
	var filesToCopy []string
	for srcPath, srcHashes := range sourceFiles {
		found := false
		for _, targetHashes := range targetFiles {
			if srcHashes.MD5 == targetHashes.MD5 && srcHashes.Blake3 == targetHashes.Blake3 {
				found = true
				break
			}
		}
		if !found {
			filesToCopy = append(filesToCopy, srcPath)
		}
	}

	util.PrintProcess("Found %d files to copy\n", len(filesToCopy))

	// Copy files that don't exist in target
	for _, srcPath := range filesToCopy {
		// Calculate relative path from source directory
		relPath, err := filepath.Rel(sourceDir, srcPath)
		if err != nil {
			return fmt.Errorf("error calculating relative path for %s: %v", srcPath, err)
		}

		// Construct destination path in backup directory
		dstPath := filepath.Join(backupDir, relPath)

		// Create directories for destination path if they don't exist
		dstDir := filepath.Dir(dstPath)
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %v", dstDir, err)
		}

		// Copy file
		util.PrintProcess("Copying %s to %s\n", srcPath, dstPath)
		if err := copyFile(srcPath, dstPath); err != nil {
			return fmt.Errorf("error copying %s to %s: %v", srcPath, dstPath, err)
		}

		// Calculate and store file info in database
		fileInfo, err := os.Stat(srcPath)
		if err != nil {
			return fmt.Errorf("error getting file info for %s: %v", srcPath, err)
		}

		absDstPath, err := filepath.Abs(dstPath)
		if err != nil {
			return fmt.Errorf("error getting absolute path for %s: %v", dstPath, err)
		}

		// Calculate path key (Blake3 of absolute path)
		key := util.CalculateBlake3String(absDstPath)

		// Calculate MD5 and Blake3 for the copied file with single file read
		blake3Hash, md5Hash, err := util.FileBlake3MD5(dstPath)
		if err != nil {
			return fmt.Errorf("error calculating hashes for %s: %v", dstPath, err)
		}

		// Get creation time
		ctime := util.GetCreationTime(fileInfo)

		// Create database record for copied file
		dbRecord := &data.FileInfo{
			Key:    key,
			Name:   filepath.Base(dstPath),
			Path:   absDstPath,
			Status: 0, // File exists
			MD5:    md5Hash,
			Blake3: blake3Hash,
			Size:   fileInfo.Size(),
			Tag:    "", // No specific tag for copied files
			MTime:  fileInfo.ModTime(),
			CTime:  ctime,
		}

		// Insert or update record in database
		if err := db.UpsertFileInfo(dbRecord); err != nil {
			return fmt.Errorf("error upserting file info for %s: %v", dstPath, err)
		}
	}

	return nil
}

// FileHashes stores MD5 and Blake3 values for a file
type FileHashes struct {
	MD5    string
	Blake3 string
}

// getFilesWithHashes traverses the directory and calculates MD5 and Blake3 for each file
// It first checks the database for existing values before calculating
func getFilesWithHashes(db *data.DB, dir string) (map[string]*FileHashes, error) {
	// First, count total files for progress tracking
	totalFiles := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip unreadable files or directories
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's the database file itself to avoid processing it
		if strings.HasSuffix(path, "fsak.db") {
			return nil
		}

		totalFiles++
		return nil
	})

	if err != nil {
		return nil, err
	}

	if totalFiles == 0 {
		return make(map[string]*FileHashes), nil
	}

	// Now process files and track progress
	files := make(map[string]*FileHashes)
	processedFiles := 0

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip unreadable files or directories
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if it's the database file itself to avoid processing it
		if strings.HasSuffix(path, "fsak.db") {
			return nil
		}

		processedFiles++
		// Get absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("error getting absolute path for %s: %v", path, err)
		}

		// First, try to get file info from database
		dbFileInfo, err := db.GetFileInfoByPath(absPath)
		if err == nil && dbFileInfo.MD5 != "" && dbFileInfo.Blake3 != "" {
			// Found in database, use stored values
			files[path] = &FileHashes{
				MD5:    dbFileInfo.MD5,
				Blake3: dbFileInfo.Blake3,
			}

			// Show progress
			percentage := float64(processedFiles) / float64(totalFiles) * 100
			util.PrintProcess("[ %d / %d (%.2f%%)]: %s\n", processedFiles, totalFiles, percentage, absPath)
		} else {
			// Not in database or missing hash values, calculate them with single file read
			blake3Hash, md5Hash, err := util.FileBlake3MD5(path)
			if err != nil {
				return fmt.Errorf("error calculating hashes for %s: %v", path, err)
			}

			// Store in database for future use
			key := util.CalculateBlake3String(absPath)

			dbRecord := &data.FileInfo{
				Key:    key,
				Name:   filepath.Base(path),
				Path:   absPath,
				Status: 0, // File exists
				MD5:    md5Hash,
				Blake3: blake3Hash,
				Size:   info.Size(),
				Tag:    "",
				MTime:  info.ModTime(),
				CTime:  util.GetCreationTime(info),
			}

			if err := db.UpsertFileInfo(dbRecord); err != nil {
				return fmt.Errorf("error upserting file info for %s: %v", path, err)
			}

			files[path] = &FileHashes{
				MD5:    md5Hash,
				Blake3: blake3Hash,
			}

			// Show progress
			percentage := float64(processedFiles) / float64(totalFiles) * 100
			util.PrintProcess("[ %d / %d (%.2f%%)]: %s\n", processedFiles, totalFiles, percentage, absPath)
		}

		return nil
	})

	return files, err
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %v", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("error creating destination file: %v", err)
	}
	defer dstFile.Close()

	// Copy contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("error copying file contents: %v", err)
	}

	// Sync to ensure data is written to disk
	err = dstFile.Sync()
	if err != nil {
		return fmt.Errorf("error syncing destination file: %v", err)
	}

	return nil
}
