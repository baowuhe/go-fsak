# go-fsak

A command-line tool for enhanced file management operations. Think of it as a Swiss Army Knife for file system operations.

## Features

- **Hash Calculation**: Calculate MD5 and Blake3 hash values of files with a single read operation
- **File Information Sync**: Traverse directories and sync file information (including hashes) to an SQLite database
- **Database Cleaning**: Clean database entries by removing records for non-existent files
- **Duplicate File Management**: Find and remove duplicate files based on hash values
- **File Merging**: Merge files between directories based on hash comparison

## Installation

```bash
# Clone the repository
git clone https://github.com/baowuhe/go-fsak.git
cd go-fsak

# Build the project
go build -o go-fsak .

# Or install directly
go install github.com/baowuhe/go-fsak@latest
```

## Usage

### Basic Commands

```bash
# Show version
go-fsak version

# Calculate hash of a file
go-fsak hash <file_path>

# Get file information and sync to database
go-fsak sync info [options] <directory_paths>

# Clean database by removing records for non-existent files
go-fsak clean info

# Find and remove duplicate files
go-fsak clean dup <folder_paths>

# Merge files from source to target directory
go-fsak merge dir --from <source_dir> --to <target_dir>
```

### Detailed Command Usage

#### Hash Command
```bash
go-fsak hash <file_path>
```
Calculate MD5 and Blake3 hash values of a file with a single read operation.

#### Sync Info Command
```bash
go-fsak sync info [options] <directory_paths>
```
Traverse one or more directories and their subdirectories, read file information, calculate MD5 and Blake3 values, and synchronize to SQLite database.

Options:
- `-t, --threads <number>`: Number of threads for calculation (default: 1)
- `-T, --tag <string>`: Tag for this batch of sync data
- `-F, --force`: Force overwrite existing data
- `-B, --blacklist <file>`: Blacklist file containing paths to exclude (supports regex)
- `-b, --batch <number>`: Number of records to batch update to SQLite database (default: 10)

#### Clean Commands
```bash
# Clean file_infos table by removing records where path points to non-existent files
go-fsak clean info

# Find and remove duplicate files
go-fsak clean dup [options] <folder_paths>
```

For duplicate file removal, you can specify:
- `-d, --deleted-save-dir <directory>`: Directory to move deleted files to (default is workspace/deleted)

#### Merge Command
```bash
go-fsak merge dir --from <source_dir> --to <target_dir>
```
Traverse source and target directories, calculate MD5 and Blake3 values, and copy files that don't exist in target based on these values.

## Data Storage

By default, go-fsak stores its data in:
- **Linux/Mac**: `$HOME/.local/share/fsak`
- **Windows**: `%LOCALAPPDATA%\fsak`

You can change this location by setting the `FSAK_WS_DIR` environment variable.

## Dependencies

- [cobra](https://github.com/spf13/cobra) - Command-line interface
- [survey](https://github.com/AlecAivazis/survey/v2) - Interactive prompts
- [gorm](https://gorm.io/) - Database ORM
- [sqlite](https://www.sqlite.org/) - Database engine
- [blake3](https://github.com/lukechampine/blake3) - Blake3 hash algorithm

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
