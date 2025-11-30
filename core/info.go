package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/baowuhe/go-fsak/data"
	"github.com/baowuhe/go-fsak/util"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info [flags] <dirs>",
	Short: "Get file information and sync to database",
	Long:  `Traverse one or more directories and their subdirectories, read file information, calculate MD5 and Blake3 values, and synchronize to SQLite database.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		threads, _ := cmd.Flags().GetInt("threads")
		tag, _ := cmd.Flags().GetString("tag")
		force, _ := cmd.Flags().GetBool("force")
		blacklistFile, _ := cmd.Flags().GetString("blacklist")
		batchSize, _ := cmd.Flags().GetInt("batch")

		dirs := args

		// Show what directories will be processed
		util.PrintProcess("Starting to process directories: %v\n", dirs)

		// Load blacklist patterns
		util.PrintProcess("Loading blacklist patterns from: %s\n", blacklistFile)
		blacklistPatterns, err := util.ReadBlacklist(blacklistFile)
		if err != nil {
			util.PrintError("Error reading blacklist: %v\n", err)
			os.Exit(1)
		}
		util.PrintProcess("Loaded %d blacklist patterns\n", len(blacklistPatterns))

		// Process directories
		processDirectories(dirs, threads, tag, force, blacklistPatterns, batchSize)
	},
}

func init() {
	syncCmd.AddCommand(infoCmd)

	infoCmd.Flags().IntP("threads", "t", 1, "Number of threads for calculation")
	infoCmd.Flags().StringP("tag", "T", "", "Tag for this batch of sync data")
	infoCmd.Flags().BoolP("force", "F", false, "Force overwrite existing data")
	infoCmd.Flags().StringP("blacklist", "B", "", "Blacklist file containing paths to exclude (supports regex)")
	infoCmd.Flags().IntP("batch", "b", 10, "Number of records to batch update to SQLite database")
}

func countFiles(dirs []string, blacklistPatterns []*regexp.Regexp) (int, error) {
	totalFiles := 0

	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Check if the file matches any blacklist pattern
			shouldSkip := false
			for _, pattern := range blacklistPatterns {
				if pattern.MatchString(path) {
					shouldSkip = true
					break
				}
			}

			if shouldSkip {
				return nil
			}

			totalFiles++

			return nil
		})

		if err != nil {
			return 0, err
		}
	}

	return totalFiles, nil
}

func processDirectories(dirs []string, threads int, tag string, force bool, blacklistPatterns []*regexp.Regexp, batchSize int) {
	// Count total files first
	util.PrintProcess("Counting files in specified directories (this may take a moment)...\n")
	totalFiles, err := countFiles(dirs, blacklistPatterns)
	if err != nil {
		util.PrintError("Error counting files: %v\n", err)
		os.Exit(1)
	}

	util.PrintProcess("Total files to process: %d\n", totalFiles)

	// Create a single database connection for all workers
	util.PrintProcess("Connecting to database...\n")
	db, err := data.Connect()
	if err != nil {
		util.PrintError("Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		sqlDB, _ := db.DB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	// Counter to track progress
	var counter struct {
		sync.Mutex
		count int
	}

	// Mutex to synchronize database operations
	var dbMutex sync.Mutex

	// Channel to send file paths to be processed
	fileCh := make(chan string, threads*2)
	// Channel to collect processed file info for batching
	resultCh := make(chan *data.FileInfo, threads*2)

	// Wait group to wait for all worker goroutines to finish
	var wg sync.WaitGroup

	// Start worker goroutines for processing files (without database operations)
	util.PrintProcess("Starting %d worker threads to process files...\n", threads)
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func(threadId int) {
			defer wg.Done()

			util.PrintProcess("Worker %d started and ready to process files\n", threadId)
			for path := range fileCh {
				fileInfo, err := processFileInfoOnly(path, tag, force, db)
				if err != nil {
					util.PrintError("Error processing file %s in worker %d: %v\n", path, threadId, err)
				} else if fileInfo != nil {
					resultCh <- fileInfo
				}
			}
			util.PrintProcess("Worker %d finished processing files\n", threadId)
		}(i) // Pass thread ID to identify each worker
	}

	// Start a goroutine to handle batching and database updates
	go func() {
		batch := make([]*data.FileInfo, 0, batchSize)
		for fileInfo := range resultCh {
			batch = append(batch, fileInfo)

			// If batch is full, save to database
			if len(batch) >= batchSize {
				dbMutex.Lock()
				for _, info := range batch {
					if err := db.UpsertFileInfo(info); err != nil {
						util.PrintError("Error upserting file info: %v\n", err)
					}
				}
				dbMutex.Unlock()

				// Update counter for all files in the batch
				counter.Lock()
				for _, info := range batch {
					counter.count++
					currentCount := counter.count
					// Calculate percentage
					percentage := 0.0
					if totalFiles > 0 {
						percentage = float64(currentCount) / float64(totalFiles) * 100
					}
					util.PrintProcess("[ %d / %d (%.2f%%)]: %s\n", currentCount, totalFiles, percentage, info.Path)
				}
				counter.Unlock()

				batch = batch[:0] // Reset batch
			}
		}

		// Save remaining items in the batch
		if len(batch) > 0 {
			dbMutex.Lock()
			for _, info := range batch {
				if err := db.UpsertFileInfo(info); err != nil {
					util.PrintError("Error upserting file info: %v\n", err)
				}
			}
			dbMutex.Unlock()

			// Update counter for all files in the final batch
			counter.Lock()
			for _, info := range batch {
				counter.count++
				currentCount := counter.count
				// Calculate percentage
				percentage := 0.0
				if totalFiles > 0 {
					percentage = float64(currentCount) / float64(totalFiles) * 100
				}
				util.PrintProcess("[ %d / %d (%.2f%%)]: %s\n", currentCount, totalFiles, percentage, info.Path)
			}
			counter.Unlock()
		}
	}()

	// Walk through directories and send files to the channel
	util.PrintProcess("Walking through directories to collect files for processing...\n")
	for i, dir := range dirs {
		util.PrintProcess("Scanning directory %d/%d: %s\n", i+1, len(dirs), dir)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Check if the file matches any blacklist pattern
			shouldSkip := false
			for _, pattern := range blacklistPatterns {
				if pattern.MatchString(path) {
					shouldSkip = true
					break
				}
			}

			if shouldSkip {
				return nil
			}

			// Send file path to be processed
			fileCh <- path

			return nil
		})

		if err != nil {
			util.PrintError("Error walking directory %s: %v\n", dir, err)
		} else {
			util.PrintProcess("Finished scanning directory: %s\n", dir)
		}
	}

	// Close the file channel to signal workers to stop
	util.PrintProcess("All files collected, closing processing channel...\n")
	close(fileCh)

	// Wait for all workers to finish
	util.PrintProcess("Waiting for all workers to complete processing...\n")
	wg.Wait()

	// Close the result channel after all workers finish
	close(resultCh)

	util.PrintSuccess("Sync operation completed.")
}

// processFileInfoOnly processes a file and returns its FileInfo struct without saving to database
func processFileInfoOnly(filePath string, tag string, force bool, db *data.DB) (*data.FileInfo, error) {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting file info for %s: %v", filePath, err)
	}

	// Calculate absolute path for database lookup
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path for %s: %v", filePath, err)
	}

	// Check if file already exists in database
	if !force {
		_, err := db.GetFileInfoByPath(absPath)
		if err == nil {
			// File exists in database and force is false, skip
			util.PrintWarning("Skipping existing file: %s\n", filePath)
			return nil, nil // Return nil to indicate file should be skipped
		} else if err != gorm.ErrRecordNotFound {
			// If there's an error other than "record not found", return the error
			return nil, fmt.Errorf("error checking if file exists in database: %v", err)
		}
		// If err is gorm.ErrRecordNotFound, the file doesn't exist in DB, so continue processing
	}

	// Calculate file key (Blake3 of absolute path)
	key := util.CalculateBlake3String(absPath)

	// Calculate MD5 and Blake3 with single file read
	blake3Hash, md5Hash, err := util.FileBlake3MD5(filePath)
	if err != nil {
		return nil, fmt.Errorf("error calculating hashes for %s: %v", filePath, err)
	}

	// Get actual creation time
	ctime := util.GetCreationTime(fileInfo)

	// Create database record
	dbRecord := &data.FileInfo{
		Key:    key,
		Name:   filepath.Base(filePath),
		Path:   absPath,
		Status: 0, // File exists
		MD5:    md5Hash,
		Blake3: blake3Hash,
		Size:   fileInfo.Size(),
		Tag:    tag,
		MTime:  fileInfo.ModTime(),
		CTime:  ctime,
	}

	return dbRecord, nil
}
