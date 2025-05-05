// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"strings"
)

// ParsedArgs holds the parsed components of the argument string.
type ParsedArgs struct {
	Owner   string
	Repo    string
	Version string // Will be "latest" or a specific tag
}

// parseArgs parses an argument string in the format owner/repo[@version].
// Supported formats:
// - owner/repo (version defaults to "latest")
// - owner/repo@latest
// - owner/repo@vX.Y.Z (or any other tag)
// Returns ParsedArgs and an error if the format is invalid.
// It also handles the "owner/repo@" case as an error.
func ParseArgs(argString string) (ParsedArgs, error) {
	var owner, repo, version string

	atIndex := strings.Index(argString, "@")

	if atIndex != -1 { // Contains "@"
		// Split into owner/repo part and version part
		parts := strings.Split(argString, "@")

		// Check for cases like "owner/repo@tag@other" or just "@"
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
			return ParsedArgs{}, fmt.Errorf("invalid owner/repo format '%s': expected owner/repo or owner/repo@version", argString)
		}
		owner = orParts[0]
		repo = orParts[1]
		version = "latest" // Default version
		Logger.Debugf("Owner: %s", owner)
		Logger.Debugf("Repo: %s", repo)
		Logger.Debugf("Version: %s", version)
	}

	// Basic validation: owner and repo shouldn't contain slashes or @
	// (Though strictly following the logic, these cases would likely
	// be caught by the split checks above, but doesn't hurt)
	if strings.Contains(owner, "/") || strings.Contains(owner, "@") {
		return ParsedArgs{}, fmt.Errorf("invalid characters in owner '%s'", owner)
	}
	if strings.Contains(repo, "/") || strings.Contains(repo, "@") {
		return ParsedArgs{}, fmt.Errorf("invalid characters in repo '%s'", repo)
	}

	return ParsedArgs{Owner: owner, Repo: repo, Version: version}, nil
}
