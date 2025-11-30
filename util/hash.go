package util

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"

	"lukechampine.com/blake3"
)

// CalculateMD5 calculates MD5 hash of a file
func CalculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CalculateCRC32 calculates CRC32 checksum of a file (legacy function)
func CalculateCRC32(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%08x", hash.Sum32()), nil
}

// CalculateMD5String calculates MD5 hash of a string
func CalculateMD5String(data string) string {
	hash := md5.New()
	hash.Write([]byte(data))
	return hex.EncodeToString(hash.Sum(nil))
}

// CalculateBlake3String calculates Blake3 hash of a string
func CalculateBlake3String(data string) string {
	hash := blake3.New(32, nil) // 32-byte output with no key
	hash.Write([]byte(data))
	return hex.EncodeToString(hash.Sum(nil))
}

// FileMD5CRC32 reads a file once and calculates both MD5 and CRC32 values (legacy function)
// Returns: MD5 (hex string), CRC32 (hex string), error
func FileMD5CRC32(path string) (md5Str string, crc32Str string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	// Two hashers
	md5Hash := md5.New()
	crc32Hash := crc32.NewIEEE()

	// Write file stream to both hashers simultaneously
	mw := io.MultiWriter(md5Hash, crc32Hash)

	// Copy entire file, underlying read happens only once
	if _, err = io.Copy(mw, f); err != nil {
		return "", "", err
	}

	// Return results
	return hex.EncodeToString(md5Hash.Sum(nil)),
		fmt.Sprintf("%d", crc32Hash.Sum32()),
		nil
}

// FileBlake3MD5 reads a file once and calculates both Blake3 and MD5 values
// Returns: Blake3 (hex string), MD5 (hex string), error
func FileBlake3MD5(path string) (blake3Str string, md5Str string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	// Two hashers
	blake3Hash := blake3.New(32, nil) // 32-byte output with no key
	md5Hash := md5.New()

	// Write file stream to both hashers simultaneously
	mw := io.MultiWriter(blake3Hash, md5Hash)

	// Copy entire file, underlying read happens only once
	if _, err = io.Copy(mw, f); err != nil {
		return "", "", err
	}

	// Return results
	return hex.EncodeToString(blake3Hash.Sum(nil)),
		hex.EncodeToString(md5Hash.Sum(nil)),
		nil
}
