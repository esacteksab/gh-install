// SPDX-License-Identifier: MIT

package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
)

var (
	// Global logger instance used across the package
	Logger *log.Logger

	// Pre-compiled regular expressions for matching OS/architecture in filenames
	osArchRegexes []*regexp.Regexp

	// Compile regex patterns once at package level
	// checksumFileRegex = regexp.MustCompile(`(?i)_?checksums?\.txt$|_?checksums?`)
)

// ParsedArgs holds the parsed components of the argument string.
// This represents the GitHub repository and version information.
type ParsedArgs struct {
	Owner   string // Repository owner (user or organization)
	Repo    string // Repository name
	Version string // Will be "latest" or a specific tag
}

// ParseArgs parses an argument string in the format owner/repo[@version].
// Supported formats:
// - owner/repo (version defaults to "latest")
// - owner/repo@latest
// - owner/repo@vX.Y.Z (or any other tag)
//
// -argString: The input string to parse.
// Returns:
//   - ParsedArgs: A struct containing the parsed components
//   - error: An error if the format is invalid
func ParseArgs(argString string) (ParsedArgs, error) {
	var owner, repo, version string

	// Check if the argument contains a version (separated by '@')
	atIndex := strings.Index(argString, "@")

	if atIndex != -1 { // Contains "@"
		// Split into owner/repo part and version part
		parts := strings.Split(argString, "@")

		// Check for cases like "owner/repo@tag@other" or just "@"
		// We expect exactly 2 parts: owner/repo and version
		if len(parts) != 2 { //nolint:mnd
			return ParsedArgs{}, fmt.Errorf(
				"invalid argument format '%s': expected owner/repo[@version]",
				argString,
			)
		}

		ownerRepoPart := parts[0]
		version = parts[1]

		// Handle the specific "owner/repo@" case where version is empty
		if version == "" {
			return ParsedArgs{}, fmt.Errorf(
				"invalid argument format '%s': missing version after '@'",
				argString,
			)
		}

		// Now parse the owner/repo part
		orParts := strings.Split(ownerRepoPart, "/")
		if len(orParts) != 2 || orParts[0] == "" || orParts[1] == "" {
			return ParsedArgs{}, fmt.Errorf(
				"invalid owner/repo format '%s' before '@': expected owner/repo",
				ownerRepoPart,
			)
		}
		owner = orParts[0]
		repo = orParts[1]
	} else { // Does not contain "@"
		// Format must be owner/repo, version is implicitly "latest"
		orParts := strings.Split(argString, "/")
		if len(orParts) != 2 || orParts[0] == "" || orParts[1] == "" {
			return ParsedArgs{}, fmt.Errorf(
				"invalid owner/repo format '%s': expected owner/repo or owner/repo@version",
				argString,
			)
		}
		owner = orParts[0]
		repo = orParts[1]
		version = "latest" // Default version
		Logger.Debugf("Owner: %s", owner)
		Logger.Debugf("Repo: %s", repo)
		Logger.Debugf("Version: %s", version)
	}

	// Basic validation: owner and repo shouldn't contain slashes or @
	// This adds additional safety checks beyond the parsing above
	if strings.Contains(owner, "/") || strings.Contains(owner, "@") {
		return ParsedArgs{}, fmt.Errorf("invalid characters in owner '%s'", owner)
	}
	if strings.Contains(repo, "/") || strings.Contains(repo, "@") {
		return ParsedArgs{}, fmt.Errorf("invalid characters in repo '%s'", repo)
	}

	return ParsedArgs{Owner: owner, Repo: repo, Version: version}, nil
}

// GetOSArch identifies the current operating system and architecture,
// and creates a set of regular expressions to match appropriate release assets.
// This prepares the system to identify assets that are compatible with the current machine.
func GetOSArch() {
	// Get the current system's OS and architecture from Go runtime
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Escape special regex characters in OS name to avoid regex pattern issues
	quotedOS := regexp.QuoteMeta(osName)

	// Create architecture mappings for common variants
	var archPatterns []string

	// Add the default Go architecture name
	archPatterns = append(archPatterns, regexp.QuoteMeta(arch))

	// Add common alternative architecture names that are used in releases
	// These handle different naming conventions used by various projects
	switch arch {
	case "amd64":
		archPatterns = append(archPatterns, "x86_64") // Common alternative for amd64
	case "386":
		archPatterns = append(archPatterns, "i386") // Common alternative for 386
	case "arm64":
		archPatterns = append(archPatterns, "aarch64") // Common alternative for arm64
	}

	// Create all combinations of OS and architecture patterns
	// This handles different formats that projects may use for naming assets
	var patterns []string
	for _, archPattern := range archPatterns {
		// Separators: -, _, / or just contains both words anywhere
		// These cover formats like: linux-amd64, linux_amd64, linux/amd64
		patterns = append(
			patterns,
			fmt.Sprintf("(?i).*%s[-_/]%s.*", quotedOS, archPattern),
		) // os<sep>arch
		patterns = append(
			patterns,
			fmt.Sprintf("(?i).*%s[-_/]%s.*", archPattern, quotedOS),
		) // arch<sep>os
		patterns = append(
			patterns,
			fmt.Sprintf(
				"(?i)(.*%s.*%s.*|.*%s.*%s.*)",
				quotedOS,
				archPattern,
				archPattern,
				quotedOS,
			),
		) // Contains both, any order
	}

	// Pre-compile all the patterns for better performance
	osArchRegexes = make([]*regexp.Regexp, len(patterns))
	Logger.Debugf("Compiling %d OS/Arch regex patterns...", len(patterns))
	for i, pattern := range patterns {
		osArchRegexes[i] = regexp.MustCompile(pattern)
		Logger.Debugf("  Pattern %d: %s", i, pattern)
	}
	Logger.Debug("OS/Arch regex compilation complete.")
}

// MatchFile checks if a filename matches the current OS and architecture patterns.
// This determines if the given file is likely compatible with the current system.
//
// -file: The filename to check against OS/architecture patterns.
// Returns: true if the file matches any of the OS/architecture patterns, false otherwise.
func MatchFile(file string) bool {
	// Ensure patterns have been compiled before checking
	if len(osArchRegexes) == 0 {
		Logger.Debug("Warning: OS/Arch regexes not initialized. Call GetOSArch() first.")
		return false // No regexes to check against
	}

	// Check if the file matches any of the pre-compiled patterns
	for i, re := range osArchRegexes {
		if re.MatchString(file) {
			Logger.Debugf("File '%s' matched pattern %d: %s", file, i, re.String())
			return true // Found a match
		}
	}

	// No match found
	Logger.Debugf("File '%s' did not match any OS/arch pattern", file)
	return false
}

// ParseChecksumFile (your existing function)
// Note: For matching, `targetFilename` should ideally be the base name of the file,
// as checksum files usually list base names.
func ParseChecksumFile(checksumFilePath, targetFilename string) (string, error) {
	safeChecksumFile := filepath.Clean(checksumFilePath)
	file, err := os.Open(safeChecksumFile)
	if err != nil {
		return "", fmt.Errorf("failed to open checksum file '%s': %w", safeChecksumFile, err)
	}
	defer file.Close() //nolint:errcheck

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") { // Skip empty lines and comments
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 { //nolint:mnd
			Logger.Debugf("skipping malformed line in checksum file: %s", line)
			continue
		}

		checksum := parts[0]
		// Filename in checksum files can be complex, often it's the last part,
		// but some formats (like BSD sum) might have filename in middle.
		// For `sha256sum` and `md5sum` output, it's usually the last non-option argument.
		// A common pattern is `checksum  filename` or `checksum *filename`.
		filenameInChecksum := parts[len(parts)-1]

		// Normalize filename found in the checksum file
		filenameInChecksum = strings.TrimPrefix(filenameInChecksum, "*") // Common for binary mode
		filenameInChecksum = strings.TrimPrefix(filenameInChecksum, "./")

		if filenameInChecksum == targetFilename {
			Logger.Debugf(
				"found expected checksum '%s' for target '%s' in checksum file '%s'",
				checksum,
				targetFilename,
				checksumFilePath,
			)
			return checksum, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading checksum file '%s': %w", checksumFilePath, err)
	}

	return "", fmt.Errorf(
		"checksum for target '%s' not found in checksum file '%s'",
		targetFilename,
		checksumFilePath,
	)
}

const (
	// DefaultAlgorithmForGenericChecksums is the algorithm assumed for generic checksum files
	// like "checksums.txt" when the algorithm cannot be derived from the filename.
	// GoReleaser uses SHA256 for its generic `_checksums.txt` file.
	DefaultAlgorithmForGenericChecksums             = "sha256"
	S_IXUSR                             os.FileMode = 0o100 // Execute by owner
	S_IXGRP                             os.FileMode = 0o010 // Execute by group
	S_IXOTH                             os.FileMode = 0o001 // Execute by others
)

// VerifyChecksum verifies a local asset against a checksum file.
// It attempts to determine the algorithm from the checksum file's name.
// If the checksum file has a generic name (e.g., "project_version_checksums.txt"),
// it uses `defaultAlgoForGeneric` (which should typically be "sha256" for GoReleaser).
// In utils/checksum.go or utils/hash.go
// func VerifyChecksum(assetPathOnDisk string, assetNameInChecksumFile string, checksumFilePath string, defaultAlgoForGeneric string) (bool, string, error)
// assetPathOnDisk: The full path to the file on the local disk whose checksum needs to be calculated.
// assetNameInChecksumFile: The name of the asset as it appears in the checksum file.
func VerifyChecksum(
	assetPathOnDisk string,
	assetNameInChecksumFile string,
	checksumFilePath string,
	defaultAlgoForGeneric string,
) (bool, string, error) {
	var determinedAlgorithm string

	algoFromExt, found := GetAlgorithmFromFilename(checksumFilePath)
	if found {
		determinedAlgorithm = algoFromExt
		Logger.Printf(
			"INFO: Using algorithm '%s' derived from checksum file extension: %s",
			determinedAlgorithm,
			checksumFilePath,
		)
	} else {
		if defaultAlgoForGeneric == "" {
			return false, "", fmt.Errorf(
				"checksum algorithm not found in checksum file name '%s' and no default algorithm provided for generic checksum files",
				checksumFilePath,
			)
		}
		determinedAlgorithm = defaultAlgoForGeneric
		Logger.Printf("INFO: Checksum file '%s' has no algorithm extension. Using default/hint: '%s'", checksumFilePath, determinedAlgorithm)
	}

	if _, err := GetHasher(determinedAlgorithm); err != nil {
		return false, determinedAlgorithm, fmt.Errorf(
			"determined algorithm '%s' is not supported: %w",
			determinedAlgorithm,
			err,
		)
	}

	// Use assetNameInChecksumFile for parsing the checksum file
	expectedChecksum, err := ParseChecksumFile(checksumFilePath, assetNameInChecksumFile)
	if err != nil {
		return false, determinedAlgorithm, fmt.Errorf(
			"failed to parse checksum file '%s' for target '%s': %w",
			checksumFilePath,
			assetNameInChecksumFile,
			err,
		)
	}

	// Use assetPathOnDisk to calculate the hash of the actual local file
	Logger.Printf(
		"INFO: Calculating %s checksum for local asset: %s",
		strings.ToUpper(determinedAlgorithm),
		assetPathOnDisk,
	)
	actualChecksum, err := HashFile(assetPathOnDisk, determinedAlgorithm) // THIS IS THE KEY CHANGE
	if err != nil {
		// This error message should use assetPathOnDisk
		return false, determinedAlgorithm, fmt.Errorf(
			"failed to calculate actual checksum for asset '%s' using %s: %w",
			assetPathOnDisk,
			determinedAlgorithm,
			err,
		)
	}

	if strings.EqualFold(expectedChecksum, actualChecksum) {
		Logger.Printf(
			"SUCCESS: Checksum VALID for '%s' (original name: '%s'). Expected: %s, Actual: %s (Algorithm: %s)",
			assetPathOnDisk,
			assetNameInChecksumFile,
			expectedChecksum,
			actualChecksum,
			determinedAlgorithm,
		)
		return true, determinedAlgorithm, nil
	}

	Logger.Errorf(
		"ERROR: Checksum INVALID for '%s' (original name: '%s'). Expected: %s, Got: %s (Algorithm: %s)",
		assetPathOnDisk,
		assetNameInChecksumFile,
		expectedChecksum,
		actualChecksum,
		determinedAlgorithm,
	)
	return false, determinedAlgorithm, fmt.Errorf(
		"checksum mismatch for asset '%s' (original name '%s'): expected '%s', got '%s'",
		assetPathOnDisk,
		assetNameInChecksumFile,
		expectedChecksum,
		actualChecksum,
	)
}

func ChmodFile(filePath string) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Fatalf("Failed to get file info for '%s': %v", filePath, err)
	}

	// 2. Get the current permission mode
	currentMode := fileInfo.Mode()
	Logger.Debugf(
		"Original permissions for '%s': %s (octal: %04o)\n",
		filePath,
		currentMode.String(),
		currentMode.Perm(),
	)

	// 3. Calculate new permissions by ORing with execute bits
	// We want to add S_IXUSR, S_IXGRP, and S_IXOTH
	// This is equivalent to `currentMode.Perm() | 0111`
	// The .Perm() method on os.FileMode gives you just the permission bits (last 9 bits)
	newMode := currentMode.Perm() | S_IXUSR | S_IXGRP | S_IXOTH
	// Alternatively, if currentMode already contains other mode bits (like os.ModeDir),
	// you'd want to preserve those and only modify the permission part:
	// newModeWithOtherBits := currentMode | S_IXUSR | S_IXGRP | S_IXOTH
	// However, os.Chmod only cares about the permission bits, so using currentMode.Perm() is safer.

	Logger.Debugf("Calculated new mode (permission part only) before Chmod: %04o\n", newMode)

	// 4. Apply the new permissions
	// os.Chmod expects an os.FileMode, which includes more than just permission bits.
	// However, it effectively only uses the permission bits part.
	// So, newMode (which is just permission bits) is fine here.
	err = os.Chmod(filePath, newMode)
	if err != nil {
		Logger.Fatalf("Failed to chmod file '%s': %v", filePath, err)
	}

	// 5. Verify new permissions (optional)
	fileInfoAfter, err := os.Stat(filePath)
	if err != nil {
		Logger.Fatalf("Failed to get file info after chmod for '%s': %v", filePath, err)
	}
	modeAfterChmod := fileInfoAfter.Mode()
	Logger.Debugf(
		"New permissions for '%s': %s (octal: %04o)\n",
		filePath,
		modeAfterChmod.String(),
		modeAfterChmod.Perm(),
	)

	// Check if execute bits are set
	if modeAfterChmod&S_IXUSR != 0 {
		Logger.Debug("Execute permission for User is SET.")
	}
	if modeAfterChmod&S_IXGRP != 0 {
		Logger.Debug("Execute permission for Group is SET.")
	}
	if modeAfterChmod&S_IXOTH != 0 {
		Logger.Debug("Execute permission for Other is SET.")
	}
}

func ParseBinaryName(assetName string) (binaryName string) {
	regex := regexp.MustCompile(`[_-]v?\d+\.\d+\.\d+`)

	match := regex.FindStringIndex(assetName)

	if match == nil {
		// If the version pattern is not found, try and parse based off simpler regex
		regex := regexp.MustCompile("[-|_]")
		result := regex.Split(assetName, -1)
		return result[0]
	}

	// The binary name is the part of the string from the beginning up to the
	// start of the matched delimiter pattern.
	return assetName[0:match[0]]
}

// helper function for testing
func resetOsArchRegexesForTesting() {
	osArchRegexes = nil
}
