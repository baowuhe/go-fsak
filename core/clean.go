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
	Long:  `Commands for cleaning database entries.`,
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

func init() {
	cleanCmd.AddCommand(cleanInfoCmd)
	cleanDupCmd.Flags().StringP("deleted-save-dir", "d", "", "Directory to move deleted files to (default is workspace/deleted)")
	cleanDupCmd.MarkFlagDirname("deleted-save-dir")
	cleanCmd.AddCommand(cleanDupCmd)
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
