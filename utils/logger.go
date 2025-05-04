// SPDX-License-Identifier: MIT

package utils

import (
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var Logger *log.Logger

// validFilenameChars = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)

// const (
// 	// Max filename length (common limit)
// 	maxFilenameLength = 255
// )

// doesExist checks if a file or directory exists at the specified path.
//
// This function uses os.Stat to determine if the path exists in the filesystem.
//
// Parameters:
//
//	path - The file system path to check for existence
//
// Returns:
//
//	bool - true if the path exists, false otherwise
// func doesExist(path string) bool {
// 	if _, err := os.Stat(path); os.IsNotExist(err) {
// 		return false
// 	}
// 	return true
// }

// createLogger creates and configures the package-level Logger instance
// based on the desired verbosity.
func CreateLogger(verbose bool) {
	var level log.Level
	var reportCaller, reportTimestamp bool
	var timeFormat string

	// Define options based on verbose
	if verbose {
		reportCaller = true
		reportTimestamp = true
		timeFormat = "2006/01/02 15:04:05"
		level = log.DebugLevel
	} else {
		reportCaller = false
		reportTimestamp = false
		timeFormat = time.Kitchen
		level = log.InfoLevel
	}

	var instanceToUse *log.Logger // Use a local variable first

	if Logger == nil {
		instanceToUse = log.NewWithOptions(os.Stderr, log.Options{
			ReportCaller:    reportCaller,
			ReportTimestamp: reportTimestamp,
			TimeFormat:      timeFormat,
			Level:           level, // Set level on creation
		})
		if instanceToUse == nil {
			os.Exit(1)
		}
	} else {
		instanceToUse = Logger // Reconfigure the existing package Logger
		instanceToUse.SetLevel(level)
		instanceToUse.SetReportTimestamp(reportTimestamp)
		instanceToUse.SetTimeFormat(timeFormat)
		instanceToUse.SetReportCaller(reportCaller)
	}

	maxWidth := 4 // Use lowercase for local var
	styles := log.DefaultStyles()
	styles.Levels[log.DebugLevel] = lipgloss.NewStyle().
		SetString(strings.ToUpper(log.DebugLevel.String())).
		Bold(true).MaxWidth(maxWidth).Foreground(lipgloss.Color("14"))
	styles.Levels[log.FatalLevel] = lipgloss.NewStyle().
		SetString(strings.ToUpper(log.FatalLevel.String())).
		Bold(true).MaxWidth(maxWidth).Foreground(lipgloss.Color("9"))
	instanceToUse.SetStyles(styles)

	Logger = instanceToUse // Assign the created/reconfigured instance

	log.SetDefault(Logger)

	// Check Logger again just to be paranoid before logging
	if Logger != nil {
		// Use the package Logger variable for the final confirmation log
		Logger.Debugf(
			"Logger configured. Verbose: %t, Level set to: %s",
			verbose,
			Logger.GetLevel(),
		)
	}
}

// validateFilePath checks if a given path string represents a simple, safe filename
// intended for use within the current directory.
// It performs checks for:
// - Emptiness
// - Directory traversal components (e.g., "..", "/") after cleaning
// - Allowed characters (alphanumeric, underscore, hyphen, period)
// - Maximum length
// - Null bytes
//
// Parameters:
//
//	path - The input path string to validate.
//
// Returns:
//
//	string - The validated simple filename (without "./") if validation succeeds.
//	error - An error detailing the validation failure if any check fails. On failure,
//	        the returned string is the original input path.
// func validateFilePath(path string) (string, error) {
// 	// --- Validate the filename parameter ---
// 	if path == "" {
// 		err := errors.New("invalid file path: filename cannot be empty")
// 		// Return original path (empty) and error
// 		return path, err
// 	}
//
// 	// 1. Basic cleaning (removes ., .., extra slashes)
// 	validatedFilename := filepath.Clean(path)
//
// 	// 2. Enforce filename only (check for separators *after* cleaning)
// 	//    Also reject "." and ".." explicitly as filenames.
// 	if filepath.Base(validatedFilename) != validatedFilename || validatedFilename == "." ||
// 		validatedFilename == ".." {
// 		err := fmt.Errorf(
// 			"invalid file path: %q must be a filename only (no directory separators)",
// 			path, // Use original path in error message for clarity
// 		)
// 		// Return original path and error
// 		return path, err
// 	}
//
// 	// 3. Check for allowed characters using regex
// 	if !validFilenameChars.MatchString(validatedFilename) {
// 		err := fmt.Errorf(
// 			"invalid file path: filename %q contains invalid characters (allowed: a-z, A-Z, 0-9, _, -, .)",
// 			validatedFilename, // Use validated filename here as it's the one checked
// 		)
// 		// Return original path and error
// 		return path, err
// 	}
//
// 	// 4. Check filename length
// 	if len(validatedFilename) > maxFilenameLength {
// 		err := fmt.Errorf(
// 			"invalid file path: filename %q exceeds maximum length of %d",
// 			validatedFilename,
// 			maxFilenameLength,
// 		)
// 		// Return original path and error
// 		return path, err
// 	}
//
// 	// 5. Check for null bytes
// 	if strings.ContainsRune(validatedFilename, '\x00') {
// 		err := fmt.Errorf("invalid file path: filename %q contains null byte", validatedFilename)
// 		// Return original path and error
// 		return path, err
// 	}
//
// 	// If all checks pass, return the validated filename (which is just the base name) and nil error
// 	return validatedFilename, nil
// }
