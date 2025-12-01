package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/baowuhe/go-fsak/data"
	"github.com/baowuhe/go-fsak/util"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// cleanCmd represents the clean command
var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean operations for database",
	Long:  `Commands for cleaning database entries and files.`,
}

// infoCmd represents the clean info command
var cleanInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Clean file_infos table by removing records where path points to non-existent files",
	Long:  `Traverse the file_infos table and remove records where the path field points to files that no longer exist.`,
	Run: func(cmd *cobra.Command, args []string) {
		err := cleanFileInfoTable()
		if err != nil {
			util.PrintError("Error during clean operation: %v\n", err)
			os.Exit(1)
		}
	},
}

// dupCmd represents the clean dup command for finding and removing duplicate files
var cleanDupCmd = &cobra.Command{
	Use:   "dup [folder paths...]",
	Short: "Find and remove duplicate files",
	Long:  `Find duplicate files in specified folder paths using MD5 and Blake3 values.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deletedSaveDir, _ := cmd.Flags().GetString("deleted-save-dir")
		err := handleDuplicateFiles(args, deletedSaveDir)
		if err != nil {
			util.PrintError("Error during duplicate file operation: %v\n", err)
			os.Exit(1)
		}
	},
}

// dirtyCmd represents the clean dirty command for removing dirty files
var cleanDirtyCmd = &cobra.Command{
	Use:   "dirty [folder paths...]",
	Short: "Remove dirty files from specified folders",
	Long:  `Remove dirty files from specified folder paths based on user selection. Dirty files are defined as: files with 0 size, files smaller than 1KB, .DS_Store files on macOS, Thumbs.db files on Windows, and empty folders.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		listOnly, _ := cmd.Flags().GetBool("list")
		deleteToDir, _ := cmd.Flags().GetString("delete-to-dir")

		if deleteToDir == "" && !listOnly {
			util.PrintError("Error: --delete-to-dir (-d) flag is required when not using --list\n")
			os.Exit(1)
		}

		err := handleDirtyFiles(args, listOnly, deleteToDir)
		if err != nil {
			util.PrintError("Error during dirty file operation: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	cleanCmd.AddCommand(cleanInfoCmd)
	cleanDupCmd.Flags().StringP("deleted-save-dir", "d", "", "Directory to move deleted files to (default is workspace/deleted)")
	cleanDupCmd.MarkFlagDirname("deleted-save-dir")
	cleanCmd.AddCommand(cleanDupCmd)

	// Add dirty command with its flags
	cleanDirtyCmd.Flags().BoolP("list", "l", false, "List dirty files only, don't delete")
	cleanDirtyCmd.Flags().StringP("delete-to-dir", "d", "", "Directory to move deleted files to (required when not using --list)")
	cleanDirtyCmd.MarkFlagDirname("delete-to-dir")
	cleanCmd.AddCommand(cleanDirtyCmd)

	rootCmd.AddCommand(cleanCmd)
}

func cleanFileInfoTable() error {
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

	// Get all file info records
	var allRecords []*data.FileInfo
	err = db.GetAllFileInfos(&allRecords)
	if err != nil {
		return fmt.Errorf("error getting all file info records: %v", err)
	}

	// Count total records
	totalRecords := len(allRecords)
	util.PrintProcess("Found %d records in file_infos table, starting validation...\n", totalRecords)

	// Check which records point to non-existent files
	var recordsToDelete []*data.FileInfo
	for i, record := range allRecords {
		// Show progress
		percentage := float64(i+1) / float64(totalRecords) * 100
		util.PrintProcess("[ %d / %d (%.2f%%)]: Checking %s\n", i+1, totalRecords, percentage, record.Path)

		// Check if file exists
		if _, err := os.Stat(record.Path); os.IsNotExist(err) {
			// File doesn't exist, mark for deletion
			recordsToDelete = append(recordsToDelete, record)
		}
	}

	// Print summary
	util.PrintProcess("Found %d records pointing to non-existent files\n", len(recordsToDelete))

	// Delete the records that point to non-existent files
	deletedCount := 0
	for _, record := range recordsToDelete {
		// Print information about the record being cleaned
		util.PrintProcess("Cleaning record ID: %d, Path: %s\n", record.ID, record.Path)

		// Delete the record
		if err := db.DeleteFileInfo(record.Key); err != nil {
			return fmt.Errorf("error deleting record with key %s: %v", record.Key, err)
		}
		deletedCount++
	}

	util.PrintSuccess("Clean operation completed. %d records deleted.\n", deletedCount)
	return nil
}

// handleDuplicateFiles finds and handles duplicate files based on MD5 and Blake3 values
func handleDuplicateFiles(folderPaths []string, deletedSaveDir string) error {
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

	// Collect all files in the specified folders
	var allFiles []string
	for _, folderPath := range folderPaths {
		files, err := getAllFilesInFolder(folderPath)
		if err != nil {
			return fmt.Errorf("error getting files from folder %s: %v", folderPath, err)
		}
		allFiles = append(allFiles, files...)
	}

	// Process each file to calculate MD5 and Blake3 values
	fileInfoMap := make(map[string]*data.FileInfo)
	totalFiles := len(allFiles)
	util.PrintProcess("Processing %d files...\n", totalFiles)

	for i, filePath := range allFiles {
		// Show progress
		percentage := float64(i+1) / float64(totalFiles) * 100
		util.PrintProcess("[ %d / %d (%.2f%%)]: Processing %s\n", i+1, totalFiles, percentage, filePath)

		// Check if file info exists in database
		dbFileInfo, err := db.GetFileInfoByPath(filePath)
		if err != nil && err != gorm.ErrRecordNotFound {
			// Some other error occurred
			return fmt.Errorf("error getting file info from database for %s: %v", filePath, err)
		}

		var fileInfo *data.FileInfo
		if err == gorm.ErrRecordNotFound || dbFileInfo == nil {
			// File info doesn't exist in database, calculate new values
			blake3Val, md5Val, err := util.FileBlake3MD5(filePath)
			if err != nil {
				util.PrintWarning("Warning: Could not calculate hash for %s: %v\n", filePath, err)
				continue
			}

			// Get file stats
			fileStat, err := os.Stat(filePath)
			if err != nil {
				util.PrintWarning("Warning: Could not get file stats for %s: %v\n", filePath, err)
				continue
			}

			// Create new FileInfo
			fileInfo = &data.FileInfo{
				Path:   filePath,
				Name:   filepath.Base(filePath),
				Key:    util.CalculateBlake3String(filePath), // Key is Blake3 of absolute path
				MD5:    md5Val,
				Blake3: blake3Val,
				Size:   fileStat.Size(),
				MTime:  fileStat.ModTime(),
				CTime:  fileStat.ModTime(), // For now, use ModTime as CTime
				Status: 0,                  // 0 means file exists
			}

			// Insert into database
			if err := db.UpsertFileInfo(fileInfo); err != nil {
				return fmt.Errorf("error inserting file info into database for %s: %v", filePath, err)
			}
		} else {
			// File info exists in database, use it
			fileInfo = dbFileInfo
		}

		fileInfoMap[filePath] = fileInfo
	}

	// Group files by MD5 and Blake3 values
	groupedFiles := make(map[string][]*data.FileInfo)
	for _, fileInfo := range fileInfoMap {
		// Create a key combining MD5 and Blake3 to identify identical files
		key := fileInfo.MD5 + ":" + fileInfo.Blake3
		groupedFiles[key] = append(groupedFiles[key], fileInfo)
	}

	// Identify duplicate groups (groups with more than 1 file)
	var duplicateGroups [][]*data.FileInfo
	for _, group := range groupedFiles {
		if len(group) > 1 {
			duplicateGroups = append(duplicateGroups, group)
		}
	}

	if len(duplicateGroups) == 0 {
		util.PrintSuccess("No duplicate files found.\n")
		return nil
	}

	util.PrintProcess("Found %d groups of duplicate files.\n", len(duplicateGroups))

	// Process each duplicate group interactively
	totalFilesProcessed := 0

	for i, group := range duplicateGroups {
		util.PrintProcess("Duplicate group %d/%d (%d files):\n", i+1, len(duplicateGroups), len(group))

		// Prepare options for user selection - sort by absolute path but show relative paths and show in requested format
		// Create a slice of indices to maintain the mapping after sorting
		indices := make([]int, len(group))
		for j := range group {
			indices[j] = j
		}

		// Sort indices based on the absolute path (for consistent ordering)
		sort.Slice(indices, func(j, k int) bool {
			return group[indices[j]].Path < group[indices[k]].Path
		})

		options := make([]string, len(group))
		sortedGroup := make([]*data.FileInfo, len(group))

		for j, idx := range indices {
			sortedGroup[j] = group[idx]
			// Use absolute path in the display format
			options[j] = fmt.Sprintf("%s | (%d bytes)", group[idx].Path, group[idx].Size)
		}

		// Ask user which files to delete
		selectedOptions, err := util.SelectMultiple(
			"Select files to delete (use space to select multiple, enter to confirm):",
			options,
		)
		if err != nil {
			return fmt.Errorf("error getting user selection for group %d: %v", i+1, err)
		}

		// Immediately process the selected files for this group
		if len(selectedOptions) > 0 {
			// Move selected files to deleted folder
			var deletedDir string
			if deletedSaveDir == "" {
				workspaceDir, err := util.GetWorkspaceDir()
				if err != nil {
					return fmt.Errorf("error getting workspace directory: %v", err)
				}
				deletedDir = filepath.Join(workspaceDir, "deleted")
			} else {
				deletedDir = deletedSaveDir
			}

			if err := os.MkdirAll(deletedDir, 0755); err != nil {
				return fmt.Errorf("error creating deleted directory: %v", err)
			}

			// Map selected options back to file paths and process them immediately
			for _, selectedOption := range selectedOptions {
				for _, fileInfo := range sortedGroup {
					// Recreate the option string using absolute path to match what the user saw
					option := fmt.Sprintf("%s | (%d bytes)", fileInfo.Path, fileInfo.Size)
					if option == selectedOption {
						// Preserve the relative path structure from the parent of the original folder (including folder name) when moving
						relPath, err := getRelativePathFromParent(fileInfo.Path, folderPaths)
						if err != nil {
							util.PrintWarning("Warning: Could not determine relative path for %s: %v\n", fileInfo.Path, err)
							relPath = filepath.Base(fileInfo.Path) // Fallback to just the filename
						}

						// Create the destination path
						destPath := filepath.Join(deletedDir, relPath)

						// Create destination directory if it doesn't exist
						destDir := filepath.Dir(destPath)
						if err := os.MkdirAll(destDir, 0755); err != nil {
							return fmt.Errorf("error creating destination directory %s: %v", destDir, err)
						}

						// Move the file
						if err := os.Rename(fileInfo.Path, destPath); err != nil {
							return fmt.Errorf("error moving file %s to %s: %v", fileInfo.Path, destPath, err)
						}

						util.PrintProcess("Moved %s to %s\n", fileInfo.Path, destPath)

						// Delete the record from file_infos table immediately after moving the file
						key := util.CalculateBlake3String(fileInfo.Path)
						if err := db.DeleteFileInfo(key); err != nil {
							// Continue with other deletions even if one fails
							util.PrintWarning("Warning: Could not delete record for file %s from database: %v\n", fileInfo.Path, err)
						} else {
							totalFilesProcessed++
						}
						break
					}
				}
			}
		}
	}

	if totalFilesProcessed == 0 {
		util.PrintSuccess("No files selected for deletion.\n")
		return nil
	}

	util.PrintSuccess("Successfully processed %d duplicate files: moved to deleted folder and removed records from database.\n", totalFilesProcessed)
	return nil
}

// getAllFilesInFolder recursively gets all files in a folder
func getAllFilesInFolder(folderPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files that can't be accessed
			return nil
		}

		if !info.IsDir() {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// getRelativePathFromParent finds the relative path of a file with respect to the parent of input folders
// This includes the input folder name in the relative path
func getRelativePathFromParent(filePath string, folderPaths []string) (string, error) {
	for _, folderPath := range folderPaths {
		if strings.HasPrefix(filePath, folderPath) {
			// Get the parent directory of the folder path
			parentDir := filepath.Dir(folderPath)
			// Calculate relative path from the parent directory
			relPath, err := filepath.Rel(parentDir, filePath)
			if err == nil {
				return relPath, nil
			}
		}
	}
	return "", fmt.Errorf("file %s does not belong to any of the specified folders", filePath)
}

// Dirty file types for user selection
type DirtyFileType int

const (
	EmptyFile DirtyFileType = iota
	SmallFile
	MacHiddenFile
	WindowsHiddenFile
	EmptyFolder
	LinuxHiddenFile
	OfficeTempFile
)

// String returns the string representation of a DirtyFileType
func (d DirtyFileType) String() string {
	switch d {
	case EmptyFile:
		return "Files with size 0"
	case SmallFile:
		return "Files smaller than 1KB"
	case MacHiddenFile:
		return "macOS .DS_Store files"
	case WindowsHiddenFile:
		return "Windows Thumbs.db files"
	case EmptyFolder:
		return "Empty folders"
	case LinuxHiddenFile:
		return "Linux/MacOS hidden files (starting with .)"
	case OfficeTempFile:
		return "Office temporary files"
	default:
		return "Unknown"
	}
}

// isDirtyFile checks if a file matches any of the dirty file criteria
func isDirtyFile(path string, info os.FileInfo) bool {
	// Check if it's a directory
	if info.IsDir() {
		return isEmptyFolder(path)
	}

	fileName := filepath.Base(path)

	// Check for empty file
	if info.Size() == 0 {
		return true
	}

	// Check for small file (< 1KB)
	if info.Size() < 1024 {
		return true
	}

	// Check for Linux/MacOS hidden files (starting with .)
	if strings.HasPrefix(fileName, ".") && fileName != "." {
		return true
	}

	// Check for Office temporary files
	if isOfficeTempFile(fileName) {
		return true
	}

	// Check for macOS .DS_Store
	if fileName == ".DS_Store" {
		return true
	}

	// Check for Windows Thumbs.db
	if fileName == "Thumbs.db" {
		return true
	}

	return false
}

// isEmptyFolder checks if a folder is empty (contains no files or only empty subfolders)
func isEmptyFolder(folderPath string) bool {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// If it's a subdirectory, check if it's empty
			if !isEmptyFolder(filepath.Join(folderPath, entry.Name())) {
				return false
			}
		} else {
			// If it's a file, the folder is not empty
			return false
		}
	}

	// If we get here, the folder is empty or only contains empty subfolders
	return true
}

// isOfficeTempFile checks if a file is an Office temporary file
func isOfficeTempFile(fileName string) bool {
	// Check for files starting with ~$ (common Office temp file pattern)
	if strings.HasPrefix(fileName, "~$") {
		return true
	}

	// Get file extension
	ext := strings.ToLower(filepath.Ext(fileName))

	// Common Office temporary file extensions
	tempExtensions := []string{".tmp", ".temp", ".asd", ".wbk", ".xlk", ".tmp2"}
	for _, tempExt := range tempExtensions {
		if ext == tempExt {
			return true
		}
	}

	// Additional check for temporary files that might not have extensions
	// but have common Office temp file patterns
	nameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	if strings.HasSuffix(nameWithoutExt, "~") {
		return true
	}

	return false
}

// findDirtyFiles finds all dirty files in the specified folders
func findDirtyFiles(folderPaths []string) (map[DirtyFileType][]string, error) {
	dirtyFiles := make(map[DirtyFileType][]string)

	for _, folderPath := range folderPaths {
		err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// Skip files that can't be accessed
				return nil
			}

			// Check if the file/directory matches any dirty criteria
			if info.IsDir() {
				if isEmptyFolder(path) {
					dirtyFiles[EmptyFolder] = append(dirtyFiles[EmptyFolder], path)
				}
			} else {
				fileName := filepath.Base(path)

				// Check for empty files
				if info.Size() == 0 {
					dirtyFiles[EmptyFile] = append(dirtyFiles[EmptyFile], path)
				}

				// Check for small files (< 1KB)
				if info.Size() > 0 && info.Size() < 1024 {
					dirtyFiles[SmallFile] = append(dirtyFiles[SmallFile], path)
				}

				// Check for Linux/MacOS hidden files (starting with .)
				if strings.HasPrefix(fileName, ".") && fileName != "." {
					dirtyFiles[LinuxHiddenFile] = append(dirtyFiles[LinuxHiddenFile], path)
				}

				// Check for Office temporary files
				if isOfficeTempFile(fileName) {
					dirtyFiles[OfficeTempFile] = append(dirtyFiles[OfficeTempFile], path)
				}

				// Check for macOS .DS_Store
				if fileName == ".DS_Store" {
					dirtyFiles[MacHiddenFile] = append(dirtyFiles[MacHiddenFile], path)
				}

				// Check for Windows Thumbs.db
				if fileName == "Thumbs.db" {
					dirtyFiles[WindowsHiddenFile] = append(dirtyFiles[WindowsHiddenFile], path)
				}
			}

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("error walking folder %s: %v", folderPath, err)
		}
	}

	return dirtyFiles, nil
}

// handleDirtyFiles handles the removal of dirty files based on user selection
func handleDirtyFiles(folderPaths []string, listOnly bool, deleteToDir string) error {
	// Define all possible dirty file types
	allDirtyTypes := []DirtyFileType{EmptyFile, SmallFile, MacHiddenFile, WindowsHiddenFile, EmptyFolder, LinuxHiddenFile, OfficeTempFile}

	// Prepare options for user selection
	options := make([]string, len(allDirtyTypes))
	for i, dirtyType := range allDirtyTypes {
		options[i] = dirtyType.String()
	}

	// Ask user which types of dirty files to clean - default to all selected
	selectedOptions, err := util.SelectMultiple(
		"Select types of dirty files to clean (default: all selected):",
		options,
	)
	if err != nil {
		return fmt.Errorf("error getting user selection: %v", err)
	}

	// If no options are selected (user deselected all), default to all
	var selectedDirtyTypes []DirtyFileType
	if len(selectedOptions) == 0 {
		// If user deselected all, default to all types
		selectedDirtyTypes = allDirtyTypes
	} else {
		// Convert selected options back to DirtyFileTypes
		for _, selectedOption := range selectedOptions {
			for _, dirtyType := range allDirtyTypes {
				if dirtyType.String() == selectedOption {
					selectedDirtyTypes = append(selectedDirtyTypes, dirtyType)
					break
				}
			}
		}
	}

	if len(selectedDirtyTypes) == 0 {
		util.PrintSuccess("No dirty file types selected. Nothing to do.\n")
		return nil
	}

	// Find all dirty files
	dirtyFiles, err := findDirtyFiles(folderPaths)
	if err != nil {
		return fmt.Errorf("error finding dirty files: %v", err)
	}

	// Filter dirty files based on user selection
	filteredDirtyFiles := make(map[DirtyFileType][]string)
	for _, dt := range selectedDirtyTypes {
		if files, exists := dirtyFiles[dt]; exists {
			filteredDirtyFiles[dt] = files
		}
	}

	// Display results
	totalFiles := 0
	for dt, files := range filteredDirtyFiles {
		if len(files) > 0 {
			util.PrintProcess("\n%s (%d):\n", dt.String(), len(files))
			for _, file := range files {
				util.PrintProcess("  %s\n", file)
			}
			totalFiles += len(files)
		}
	}

	if totalFiles == 0 {
		util.PrintSuccess("No dirty files found matching your selection.\n")
		return nil
	}

	util.PrintProcess("\nTotal dirty files found: %d\n", totalFiles)

	// If list only, exit here
	if listOnly {
		util.PrintSuccess("Listing only - no files were deleted.\n")
		return nil
	}

	// Ask for confirmation before deletion
	confirmed, err := util.Confirm("Do you want to proceed with deletion? (y/N)", false)
	if err != nil {
		return fmt.Errorf("error getting confirmation: %v", err)
	}

	if !confirmed {
		util.PrintSuccess("Operation cancelled by user.\n")
		return nil
	}

	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(deleteToDir, 0755); err != nil {
		return fmt.Errorf("error creating delete directory %s: %v", deleteToDir, err)
	}

	// Process deletions
	filesDeleted := 0
	for _, files := range filteredDirtyFiles {
		for _, file := range files {
			// Create destination path preserving directory structure
			relPath, err := filepath.Rel(folderPaths[0], file)
			if err != nil {
				// If we can't get relative path, just use the filename
				relPath = filepath.Base(file)
			}
			destPath := filepath.Join(deleteToDir, relPath)

			// For directories, we need to make sure the destination path is unique
			if info, err := os.Stat(file); err == nil && info.IsDir() {
				// For directories, append a suffix to avoid conflicts
				counter := 1
				originalDestPath := destPath
				for {
					if _, err := os.Stat(destPath); os.IsNotExist(err) {
						break
					}
					ext := filepath.Ext(originalDestPath)
					name := strings.TrimSuffix(originalDestPath, ext)
					destPath = fmt.Sprintf("%s_%d%s", name, counter, ext)
					counter++
				}
			}

			// Create destination directory if needed
			destDir := filepath.Dir(destPath)
			if err := os.MkdirAll(destDir, 0755); err != nil {
				util.PrintError("Error creating destination directory for %s: %v\n", file, err)
				continue
			}

			// Move the file/directory to the delete directory
			if err := os.Rename(file, destPath); err != nil {
				util.PrintError("Error moving %s to %s: %v\n", file, destPath, err)
				continue
			}

			util.PrintProcess("Moved %s to %s\n", file, destPath)
			filesDeleted++
		}
	}

	util.PrintSuccess("Successfully moved %d dirty files to %s\n", filesDeleted, deleteToDir)
	return nil
}
