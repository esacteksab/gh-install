// SPDX-License-Identifier: MIT

package utils

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

// CreateLogger creates and configures the package-level Logger instance
// based on the desired verbosity. This function can create a new logger
// or reconfigure an existing one.
//
// -verbose: Boolean indicating if debug-level logging should be enabled.
func CreateLogger(verbose bool) {
	var level log.Level
	var reportCaller, reportTimestamp bool
	var timeFormat string

	// Configure logger options based on verbose flag
	if verbose {
		// In verbose mode, show more detailed log information
		reportCaller = true                // Include the caller's file and line number
		reportTimestamp = true             // Include timestamps in log messages
		timeFormat = "2006/01/02 15:04:05" // Use standard date/time format
		level = log.DebugLevel             // Show debug-level messages
	} else {
		// In normal mode, show minimal log information
		reportCaller = false    // Don't include caller information
		reportTimestamp = false // Don't include timestamps
		timeFormat = ""         // No time format needed
		level = log.InfoLevel   // Only show info-level and higher messages
	}

	// Use a local variable first before assigning to the package-level Logger
	var instanceToUse *log.Logger

	// Create a new logger if one doesn't exist yet
	if Logger == nil {
		instanceToUse = log.NewWithOptions(os.Stderr, log.Options{
			ReportCaller:    reportCaller,    // Whether to include caller info
			ReportTimestamp: reportTimestamp, // Whether to show timestamps
			TimeFormat:      timeFormat,      // Format for timestamps
			Level:           level,           // Minimum log level to display
		})

		// Safety check for logger creation
		if instanceToUse == nil {
			os.Exit(1) // Exit if logger creation failed
		}
	} else {
		// Reconfigure the existing logger if it already exists
		instanceToUse = Logger
		instanceToUse.SetLevel(level)                     // Update log level
		instanceToUse.SetReportTimestamp(reportTimestamp) // Update timestamp display
		instanceToUse.SetTimeFormat(timeFormat)           // Update time format
		instanceToUse.SetReportCaller(reportCaller)       // Update caller reporting
	}

	// Configure custom styles for log levels
	maxWidth := 4 // Width for level display in log messages
	styles := log.DefaultStyles()

	// Customize debug level style - cyan color
	styles.Levels[log.DebugLevel] = lipgloss.NewStyle().
		SetString(strings.ToUpper(log.DebugLevel.String())).           // "DEBUG"
		Bold(true).MaxWidth(maxWidth).Foreground(lipgloss.Color("14")) // Cyan color

	// Customize fatal level style - red color
	styles.Levels[log.FatalLevel] = lipgloss.NewStyle().
		SetString(strings.ToUpper(log.FatalLevel.String())).          // "FATAL"
		Bold(true).MaxWidth(maxWidth).Foreground(lipgloss.Color("9")) // Red color

	// Apply the styles to the logger
	instanceToUse.SetStyles(styles)

	// Set the package-level Logger variable to our configured instance
	Logger = instanceToUse

	// Also set this as the default logger for the log package
	log.SetDefault(Logger)

	// Final verification that Logger was properly initialized
	if Logger != nil {
		// Log the configuration at debug level
		// This will only be visible if verbose mode is enabled
		Logger.Debugf(
			"Logger configured. Verbose: %t, Level set to: %s",
			verbose,
			Logger.GetLevel(),
		)
	}
}

// The commented out code below was in the original file.
// It's preserved here for reference but is not currently used.
//
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
