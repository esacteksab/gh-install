// SPDX-License-Identifier: MIT

package utils

import (
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	// For algorithms not in the standard library but used by GoReleaser
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/sha3"
)

// algorithmExts as provided in your IsChecksumFile logic
// This maps file extensions to a boolean indicating they are recognized checksum extensions.
var algorithmExts = map[string]bool{
	".sha256":   true,
	".sha512":   true,
	".sha1":     true,
	".crc32":    true,
	".md5":      true,
	".sha224":   true,
	".sha384":   true,
	".sha3-256": true,
	".sha3-512": true,
	".sha3-224": true,
	".sha3-384": true,
	".blake2s":  true,
	".blake2b":  true,
}

// This regex is used by IsChecksumFile to identify general checksum files like "checksums.txt".
var checksumFileRegex = regexp.MustCompile(
	`(?i)(^(sha\d*sums?(\.txt)?|md5sums?(\.txt)?|checksums\.txt)$|checksums?(\.txt)?)`,
)

// GetHasher returns a new hash.Hash instance for the given algorithm,
// mirroring GoReleaser's supported algorithms and their specific instantiations.
func GetHasher(algorithm string) (hash.Hash, error) { //nolint:gocyclo
	var h hash.Hash
	var err error // Required for blake2b/s which can return an error on New

	// Normalize algorithm name to lowercase to match GoReleaser's case and for robustness
	algoLower := strings.ToLower(algorithm)

	switch algoLower {
	case "blake2b":
		h, err = blake2b.New512(nil) // GoReleaser uses New512 for "blake2b"
		if err != nil {
			return nil, fmt.Errorf("failed to create blake2b hasher: %w", err)
		}
	case "blake2s":
		h, err = blake2s.New256(nil) // GoReleaser uses New256 for "blake2s"
		if err != nil {
			return nil, fmt.Errorf("failed to create blake2s hasher: %w", err)
		}
	case "crc32":
		h = crc32.NewIEEE()
	case "md5":
		h = md5.New() //nolint:gosec
	case "sha224":
		h = sha256.New224()
	case "sha384":
		h = sha512.New384()
	case "sha256":
		h = sha256.New()
	case "sha1":
		h = sha1.New() //nolint:gosec
	case "sha512":
		h = sha512.New()
	case "sha3-224":
		h = sha3.New224()
	case "sha3-384":
		h = sha3.New384()
	case "sha3-256":
		h = sha3.New256()
	case "sha3-512":
		h = sha3.New512()
	default:
		return nil, fmt.Errorf("invalid or unsupported hash algorithm: %s", algorithm)
	}
	return h, nil
}

// HashFile calculates the specified checksum of a file.
// Returns the hex-encoded checksum string and an error if any occurs.
func HashFile(assetPath, algorithm string) (string, error) {
	safeFile := filepath.Clean(assetPath)
	file, err := os.Open(safeFile)
	if err != nil {
		return "", fmt.Errorf("failed to open file '%s' for hashing: %w", safeFile, err)
	}
	defer file.Close() //nolint:errcheck

	hasher, err := GetHasher(algorithm)
	if err != nil {
		// Error from GetHasher already includes algorithm info
		return "", fmt.Errorf("failed to initialize hasher for file '%s': %w", safeFile, err)
	}

	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf(
			"failed to read data from file '%s' for hashing with %s: %w",
			safeFile,
			algorithm,
			err,
		)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	Logger.Debugf(
		"%s checksum for '%s': %s",
		strings.ToUpper(algorithm),
		safeFile,
		checksum,
	) // Replace with your Logger
	return checksum, nil
}

// IsChecksumFile checks if the given filename indicates a checksum file
// based on common checksum manifest filenames or recognized algorithm extensions.
func IsChecksumFile(filePath string) bool {
	// Pattern 1: Check for common checksum manifest filenames (e.g., "checksums.txt")
	// We use filepath.Base to only check the filename itself, not the directory path.
	if checksumFileRegex.MatchString(filepath.Base(filePath)) {
		return true
	}

	// Pattern 2: Check for algorithm extension (e.g., "archive.tar.gz.sha256")
	ext := filepath.Ext(strings.ToLower(filePath)) // Ensure lowercase for map lookup
	if _, ok := algorithmExts[ext]; ok {
		return true
	}

	return false
}

// GetAlgorithmFromFilename attempts to derive the hash algorithm name (e.g., "sha256")
// from a filename, typically by looking at its extension (e.g., ".sha256").
// It returns the algorithm name and true if a recognized algorithm extension is found.
func GetAlgorithmFromFilename(filename string) (string, bool) {
	ext := filepath.Ext(strings.ToLower(filename)) // Ensure lowercase for map lookup
	if isKnownExt, ok := algorithmExts[ext]; ok && isKnownExt {
		// Remove the leading dot from the extension to get the algorithm name
		// e.g., ".sha256" -> "sha256"
		return strings.TrimPrefix(ext, "."), true
	}
	return "", false
}

// ListSupportedAlgorithms returns a slice of algorithm name strings that GetHasher supports.
func ListSupportedAlgorithms() []string {
	return []string{
		"blake2b", "blake2s", "crc32", "md5", "sha224", "sha384",
		"sha256", "sha1", "sha512", "sha3-224", "sha3-384",
		"sha3-256", "sha3-512",
	}
}
