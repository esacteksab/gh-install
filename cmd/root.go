// SPDX-License-Identifier: MIT
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/google/go-github/v71/github"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/esacteksab/gh-install/ghclient"
	"github.com/esacteksab/gh-install/utils"
)

// Build information variables populated at build time
var (
	Version string // Application version
	Date    string // Build date
	Commit  string // Git commit hash
	BuiltBy string // Builder identifier
	green   = color.New(color.FgGreen).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
)

// Asset represents a successfully downloaded and verified release asset
type Asset struct {
	Name     string // Original filename of the downloaded asset from GitHub
	Path     string // Local path where the asset was saved
	MIMEType string // MIME content type of the asset
}

// Environment variable name for enabling debug logging during initialization
const ghInstallInitDebugEnv = "GH_INSTALL_INIT_DEBUG"

// init is automatically called when the package is loaded.
// It initializes the logger and sets version information for the command.
func init() {
	// Create initial Logger with default log level (Info)
	utils.CreateLogger(false)

	// Format and set version information for the command
	rootCmd.Version = utils.BuildVersion(Version, Commit, Date, BuiltBy)

	// Customize how the version is printed when `gh install --version` is run
	rootCmd.SetVersionTemplate(`{{printf "Version %s" .Version}}`)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
//
// Returns: Does not return a value, but exits the program with status code 1 if an error occurs.
func Execute() {
	// Check environment variable for initial verbosity level
	debugEnvVal := os.Getenv(ghInstallInitDebugEnv)

	// Parse boolean value from environment - accepts "true", "TRUE", "1", etc.
	initialVerbose, _ := strconv.ParseBool(debugEnvVal)
	// If parsing fails (empty string, invalid value), initialVerbose remains false

	// Create/reconfigure logger based on environment variable
	utils.CreateLogger(initialVerbose)

	// This debug message will only appear if GH_INSTALL_INIT_DEBUG is set to true
	utils.Logger.Debugf(
		"Initial logger created in Execute(). Initial Verbose based on %s: %t",
		ghInstallInitDebugEnv,
		initialVerbose,
	)

	// Initialize OS and architecture detection for matching release assets
	utils.GetOSArch()

	// Execute the root command. If an error occurs, exit with error code
	if err := rootCmd.Execute(); err != nil {
		// Use the logger for consistency, though Printf is also fine here.
		utils.Logger.Errorf("error: %s", err)
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands.
// It's the entry point for the `gh install` command.
var rootCmd = &cobra.Command{
	Use:           "install owner/repo[@version]",
	SilenceUsage:  true,
	SilenceErrors: true,
	Short:         "gh installs binaries published on GitHub releases.",
	Long: `gh installs binaries published on GitHub releases.
Detects Operating System and Architecture to download and
install the appropriate binary. Includes checksum verification if available.`,
	Args: cobra.ExactArgs(1), // Ensure exactly one argument is provided
	RunE: func(cmd *cobra.Command, args []string) error {
		// Expect the first argument to be in the format owner/repo[@version]
		a := args[0]

		// Parse the argument string into owner, repo, and optional version
		pa, err := utils.ParseArgs(a)
		if err != nil {
			// Return the error to cobra for consistent error handling
			return fmt.Errorf("invalid argument: %w", err)
		}

		// Create a background context for API calls
		ctx := context.Background()

		// Initialize the GitHub client
		client, err := ghclient.NewClient(ctx)
		if err != nil {
			utils.Logger.Errorf("Failed to initialize GitHub client: %v", err)
			return fmt.Errorf("failed to initialize GitHub client: %v", err)
		}

		// Check and log the current GitHub API rate limit
		ghclient.CheckRateLimit(ctx, client)

		var assets []*github.ReleaseAsset
		var releaseTag string // To store the actual tag name (latest or specific)

		// Get assets based on version specified
		if pa.Version == "latest" || pa.Version == "" {
			utils.Logger.Infof("Fetching assets for latest release of %s/%s", pa.Owner, pa.Repo)
			release, err := getLatestRelease(ctx, client, pa.Owner, pa.Repo)
			if err != nil {
				return fmt.Errorf("could not get latest release: %w", err)
			}
			assets = release.Assets
			releaseTag = release.GetTagName() // Get the actual tag name for "latest"
			utils.Logger.Infof("Latest release tag: %s", releaseTag)
		} else {
			utils.Logger.Infof("Fetching assets for release tag '%s' of %s/%s", pa.Version, pa.Owner, pa.Repo)
			release, err := getTaggedRelease(ctx, client, pa.Owner, pa.Repo, pa.Version)
			if err != nil {
				return fmt.Errorf("could not get release for tag '%s': %w", pa.Version, err)
			}
			assets = release.Assets
			releaseTag = release.GetTagName() // Tag name is the one requested
		}

		if len(assets) == 0 {
			return fmt.Errorf("no assets found for release '%s'", releaseTag)
		}

		// Download the appropriate asset for current OS/architecture and verify checksum
		downloadedAsset, err := findDownloadAndVerifyAsset(
			ctx,
			client,
			pa.Owner,
			pa.Repo,
			assets,
			http.DefaultClient,
		)
		if err != nil {
			// Error already logged within the function, just return it
			return err // Cobra will print this error
		}

		// Success
		utils.Logger.Infof("Successfully downloaded and verified: %s", downloadedAsset.Name)
		utils.Logger.Infof("Asset saved to: %s", downloadedAsset.Path)
		utils.Logger.Debugf("Asset MIME Type: %s", downloadedAsset.MIMEType)

		// Add subsequent steps here (e.g., unpacking, moving to bin)
		utils.Logger.Info(">>> Next steps (unpacking, installation) are not yet implemented. <<<")

		return nil // Indicate success to Cobra
	},
}

func getLatestRelease(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
) (*github.RepositoryRelease, error) {
	release, resp, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("repository %s/%s not found or has no releases", owner, repo)
		}
		return nil, fmt.Errorf(
			"failed to get latest release: %w (Rate Limit: %s)",
			err,
			resp.Rate.String(),
		)
	}
	if release == nil {
		return nil, errors.New("received nil release object from GitHub API")
	}
	return release, nil
}

// getTaggedRelease retrieves the full release object for a specific tag.
func getTaggedRelease(
	ctx context.Context,
	client *github.Client,
	owner, repo, tag string,
) (*github.RepositoryRelease, error) {
	release, resp, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("release with tag '%s' not found in %s/%s", tag, owner, repo)
		}
		return nil, fmt.Errorf(
			"failed to get release by tag '%s': %w (Rate Limit: %s)",
			tag,
			err,
			resp.Rate.String(),
		)
	}
	if release == nil {
		return nil, fmt.Errorf("received nil release object for tag '%s' from GitHub API", tag)
	}
	return release, nil
}

// downloadAndSaveAsset downloads a specific release asset and saves it locally.
// Returns the path to the saved file and any error encountered.
func downloadAndSaveAsset(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	asset *github.ReleaseAsset,
	httpClient *http.Client,
) (filePath string, err error) { // Changed return signature
	if asset == nil || asset.Name == nil || asset.ID == nil || asset.Size == nil {
		return "", errors.New("asset has missing information (name, id, or size)")
	}

	assetName := *asset.Name
	assetID := *asset.ID
	assetSize := *asset.Size

	utils.Logger.Debugf(
		"Initiating download for asset: %s (ID: %d, Size: %d)",
		assetName,
		assetID,
		assetSize,
	)

	rc, redirectURL, err := client.Repositories.DownloadReleaseAsset(
		ctx,
		owner,
		repo,
		assetID,
		httpClient,
	)
	if err != nil {
		return "", fmt.Errorf("error initiating download for '%s': %w", assetName, err)
	}
	if rc == nil {
		// This case should ideally not happen if err is nil, but good to check.
		// If redirectURL is present, the http client usually handles it automatically.
		// If DownloadReleaseAsset returns a redirectURL, it means the underlying http.Client
		// was configured *not* to follow redirects, which isn't the default.
		if redirectURL != "" {
			utils.Logger.Warnf(
				"Download for '%s' resulted in a redirect URL (%s) but no reader. This might indicate an issue with the HTTP client configuration.",
				assetName,
				redirectURL,
			)
			return "", fmt.Errorf(
				"download resulted in redirect URL '%s' instead of data stream",
				redirectURL,
			)
		}
		return "", fmt.Errorf(
			"download request for '%s' returned no data stream and no error",
			assetName,
		)
	}
	defer rc.Close() //nolint:errcheck

	// If we got a redirect URL *and* a reader, log it but proceed.
	if redirectURL != "" {
		utils.Logger.Warnf(
			"Received both a reader and a redirect URL ('%s') for asset '%s'. Proceeding with download from reader.",
			redirectURL,
			assetName,
		)
	}

	// Save the asset data to a local file
	localPath := filepath.Clean(filepath.Base(assetName)) // Save in current dir, clean the name
	err = saveAssetToFile(rc, localPath, assetName, int64(assetSize))
	if err != nil {
		return "", err // Error already contains context
	}

	// Return the path where the file was saved
	return localPath, nil
}

// saveAssetToFile saves asset data from a reader to a local file with progress display.
// Takes the localPath where the file should be created.
func saveAssetToFile(rc io.ReadCloser, localPath, displayName string, assetSize int64) error {
	utils.Logger.Debugf("Saving asset '%s' to local path '%s'", displayName, localPath)

	// Create the output file
	file, err := os.Create(localPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", localPath, err)
	}
	// Use a flag to ensure file closing happens even if Copy fails mid-way
	var fileClosed bool
	defer func() {
		if !fileClosed {
			file.Close() //nolint:errcheck,gosec
		}
	}()

	// Setup progress bar using the display name
	bar := progressbar.NewOptions64(
		assetSize,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(35), //nolint:mnd
		progressbar.OptionSetDescription(
			fmt.Sprintf("[cyan]Downloading %s...[reset]", displayName),
		),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionClearOnFinish(),
	)

	// Copy data from the reader to both the file and the progress bar
	_, copyErr := io.Copy(io.MultiWriter(file, bar), rc)

	// Close the file explicitly *before* checking copyErr to ensure data is flushed
	closeErr := file.Close()
	fileClosed = true // Mark file as closed

	// Handle errors
	if copyErr != nil {
		utils.Logger.Errorf("Error during download/copy for '%s': %v", displayName, copyErr)
		// Attempt cleanup on copy error
		_ = os.Remove(localPath)
		return fmt.Errorf("error saving data for '%s': %w", displayName, copyErr)
	}

	if closeErr != nil {
		utils.Logger.Errorf("Error closing file '%s' after download: %v", localPath, closeErr)
	}

	utils.Logger.Printf(green("✔")+" Successfully downloaded %s", displayName)
	return nil
}

// findDownloadAndVerifyAsset identifies the main asset and checksum file,
// downloads them, and verifies.
// gocyclo:ignore // Function follows a clear sequential process; further
// extraction may reduce readability.
// funlnen:ignore // 105 > 100, close enough
func findDownloadAndVerifyAsset( //nolint:gocyclo,funlen
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	assets []*github.ReleaseAsset,
	httpClient *http.Client,
) (Asset, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	var mainAssetToDownload *github.ReleaseAsset
	var checksumAssetToDownload *github.ReleaseAsset

	utils.Logger.Debugf(
		"Scanning %d assets to find matching binary/archive and checksum file...",
		len(assets),
	)

	// Identify potential assets
	for _, asset := range assets {
		if asset == nil || asset.Name == nil || asset.ID == nil {
			utils.Logger.Debug("Skipping asset with missing name or ID.")
			continue
		}
		assetName := *asset.Name

		if utils.IsChecksumFile(assetName) {
			if checksumAssetToDownload == nil {
				utils.Logger.Debugf("Found potential checksum file: %s", assetName)
				checksumAssetToDownload = asset
			} else {
				utils.Logger.Warnf("Found multiple potential checksum files. Using the first one found: '%s'. Ignoring '%s'.", *checksumAssetToDownload.Name, assetName)
			}
			continue
		}

		if utils.MatchFile(assetName) {
			if mainAssetToDownload == nil {
				utils.Logger.Debugf("Found potential main asset matching OS/Arch: %s", assetName)
				mainAssetToDownload = asset
			} else {
				utils.Logger.Warnf("Found multiple assets matching OS/Arch. Using the first one found: '%s'. Ignoring '%s'.", *mainAssetToDownload.Name, assetName)
			}
		}
	}

	// Validate findings
	if mainAssetToDownload == nil {
		utils.Logger.Error(
			"No asset matching the current OS/Architecture was found in the release.",
		)
		return Asset{}, errors.New("no suitable asset found for download")
	}

	utils.Logger.Infof("Selected main asset for download: %s", *mainAssetToDownload.Name)
	if checksumAssetToDownload != nil {
		utils.Logger.Infof("Selected checksum file: %s", *checksumAssetToDownload.Name)
	} else {
		utils.Logger.Warn(yellow("No checksum file found in the release assets. Proceeding without verification."))
	}

	// Download Main Asset
	mainAssetPath, err := downloadAndSaveAsset(
		ctx,
		client,
		owner,
		repo,
		mainAssetToDownload,
		httpClient,
	)
	if err != nil {
		return Asset{}, fmt.Errorf(
			"failed to download main asset '%s': %w",
			*mainAssetToDownload.Name,
			err,
		)
	}

	// Download Checksum File and Verify (if found)
	if checksumAssetToDownload != nil {
		checksumAssetPath, err := downloadAndSaveAsset(
			ctx,
			client,
			owner,
			repo,
			checksumAssetToDownload,
			httpClient,
		)
		if err != nil {
			// Log error, warn user, but DO NOT remove mainAssetPath here.
			// Verification is skipped, leaving the potentially unverifiable main asset.
			// Allowing it to continue to exist allows for investigation by the end user.
			// May make sense to put this functionality behind `--verbose` and != Verbose, os.Remove
			utils.Logger.Errorf(
				red(
					"Failed to download checksum file '%s': %v. Checksum verification will be skipped.",
				),
				*checksumAssetToDownload.Name,
				err,
			)
			utils.Logger.Warnf(
				yellow(
					"Verification skipped due to checksum file download error. The integrity of '%s' is NOT confirmed.",
				),
				mainAssetPath,
			)
			// Proceed without verification in this case
		} else {
			err = verifyAssetChecksum(mainAssetPath, *mainAssetToDownload.Name, checksumAssetPath)
			if err != nil {
				// Verification failed, helper function handled logging and cleanup.
				// Return the error it provided.
				return Asset{}, err
			}
			// Verification successful, checksum file removed by helper. Continue.
		}
	}

	// --- 5. Return info about the main asset ---
	return Asset{
		Name:     *mainAssetToDownload.Name,
		Path:     mainAssetPath,
		MIMEType: *mainAssetToDownload.ContentType, // Get ContentType here
	}, nil
} // End of findDownloadAndVerifyAsset

// verifyAssetChecksum handles the process of verifying the main asset against its checksum file.
// It parses the checksum file, calculates the hash of the main asset, compares them,
// and cleans up downloaded files based on the outcome.
// Returns nil on successful verification, otherwise returns an error.
func verifyAssetChecksum(mainAssetPath, mainAssetName, checksumAssetPath string) error {
	utils.Logger.Info("Verifying checksum...")

	// 1. Parse the checksum file to find the expected checksum
	expectedChecksum, err := utils.ParseChecksumFile(checksumAssetPath, mainAssetName)
	if err != nil {
		utils.Logger.Errorf(
			"Failed to find or parse checksum for '%s' in '%s': %v",
			mainAssetName,
			checksumAssetPath,
			err,
		)
		// Clean up both files because verification cannot proceed
		_ = os.Remove(mainAssetPath)
		_ = os.Remove(checksumAssetPath)
		return fmt.Errorf(
			"checksum verification failed: could not find entry for '%s' in the checksum file",
			mainAssetName,
		)
	}

	// 2. Calculate the actual checksum of the downloaded main asset
	actualChecksum, err := utils.HashFile(
		mainAssetPath,
	) // Assumes utils.HashFile returns (string, error)
	if err != nil {
		utils.Logger.Errorf(
			"Failed to calculate checksum for downloaded file '%s': %v",
			mainAssetPath,
			err,
		)
		// Clean up both files because we can't verify
		_ = os.Remove(mainAssetPath)
		_ = os.Remove(checksumAssetPath)
		return fmt.Errorf(
			"checksum verification failed: could not hash downloaded file '%s'",
			mainAssetPath,
		)
	}

	// 3. Compare checksums (case-insensitive is safer)
	if !strings.EqualFold(expectedChecksum, actualChecksum) {
		utils.Logger.Errorf("CHECKSUM MISMATCH for %s!", mainAssetName)
		utils.Logger.Errorf("  Expected: %s", expectedChecksum)
		utils.Logger.Errorf("  Actual:   %s", actualChecksum)
		// Clean up both files due to mismatch
		_ = os.Remove(mainAssetPath)
		_ = os.Remove(checksumAssetPath)
		return errors.New(red("checksum mismatch - downloaded file is corrupt or incorrect"))
	}

	// 4. Success
	utils.Logger.Print(green("✔") + " Checksum verified successfully.")

	// 5. Clean up the checksum file (optional, but good practice)
	err = os.Remove(checksumAssetPath)
	if err != nil {
		utils.Logger.Warnf(
			"Could not remove checksum file '%s' after verification: %v",
			checksumAssetPath,
			err,
		)
		// Do not return an error here, verification itself was successful
	} else {
		utils.Logger.Debugf("Removed checksum file: %s", checksumAssetPath)
	}

	return nil // Verification successful
}
