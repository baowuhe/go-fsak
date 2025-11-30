package data

import (
	"time"

	"github.com/baowuhe/go-fsak/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// FileInfo represents file information
type FileInfo struct {
	ID     int64     `gorm:"primaryKey;autoIncrement"`
	Key    string    `gorm:"type:varchar(64);not null;unique;index"`
	Name   string    `gorm:"type:text;not null;index"`
	Path   string    `gorm:"type:text;not null;index"`
	Status int       `gorm:"type:tinyint;not null;default:0"`
	MD5    string    `gorm:"type:varchar(32);index"`
	Blake3 string    `gorm:"type:varchar(64);index"` // Blake3 hash (64 hex chars for 32-byte hash)
	Size   int64     `gorm:"type:bigint"`
	Tag    string    `gorm:"type:varchar(32)"`
	MTime  time.Time `gorm:"column:mtime"`
	CTime  time.Time `gorm:"column:ctime"`
}

// TableName specifies the table name for FileInfo
func (FileInfo) TableName() string {
	return "tb_file_infos"
}

// DB is a wrapper around gorm.DB
type DB struct {
	*gorm.DB
}

// GetDBPath returns the path to the database file
func GetDBPath() (string, error) {
	return util.GetDBPath()
}

// Connect connects to the SQLite database
func Connect() (*DB, error) {
	dbPath, err := GetDBPath()
	if err != nil {
		return nil, err
	}

	// Open database with GORM - configure SQLite for better concurrent access
	dsn := dbPath + "?_busy_timeout=30000&_journal_mode=WAL&_sync=0&_cache_size=10000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Silent by default
	})
	if err != nil {
		return nil, err
	}

	// Configure the underlying SQL database for better concurrency
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set connection pool parameters
	sqlDB.SetMaxOpenConns(1)    // Limit to 1 connection to avoid locking issues
	sqlDB.SetMaxIdleConns(1)    // Only keep 1 idle connection
	sqlDB.SetConnMaxLifetime(0) // Connections can live indefinitely

	// Auto-migrate the schema - this creates the table if it doesn't exist and updates it if needed
	if err := db.AutoMigrate(&FileInfo{}); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// GetFileInfoByPath retrieves file info by path
func (db *DB) GetFileInfoByPath(path string) (*FileInfo, error) {
	var fileInfo FileInfo
	result := db.Where("path = ?", path).First(&fileInfo)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, result.Error
		}
		return nil, result.Error
	}

	return &fileInfo, nil
}

// UpsertFileInfo creates or updates file info in the database
func (db *DB) UpsertFileInfo(fileInfo *FileInfo) error {
	// For SQLite, we can use the Assign method with FirstOrCreate or use Save
	// First try to find if the record exists based on the key
	var existing FileInfo
	result := db.Where("key = ?", fileInfo.Key).First(&existing)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Record doesn't exist, create it
			return db.Create(fileInfo).Error
		}
		// Some other error occurred
		return result.Error
	}

	// Record exists, update it
	fileInfo.ID = existing.ID // Keep the existing ID
	return db.Save(fileInfo).Error
}

// CountAllFiles returns the count of all files in the database
func (db *DB) CountAllFiles() (int64, error) {
	var count int64
	result := db.Model(&FileInfo{}).Count(&count)
	return count, result.Error
}

// GetAllFileInfos retrieves all file info records
func (db *DB) GetAllFileInfos(records *[]*FileInfo) error {
	return db.Find(records).Error
}

// DeleteFileInfo deletes file info by key
func (db *DB) DeleteFileInfo(key string) error {
	return db.Where("key = ?", key).Delete(&FileInfo{}).Error
}
