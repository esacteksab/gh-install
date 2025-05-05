// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
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
