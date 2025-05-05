// SPDX-License-Identifier: MIT
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/go-github/v71/github"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/esacteksab/gh-install/ghclient"
	"github.com/esacteksab/gh-install/utils"
)

// Build information variables populated at build time
var (
	Version string      // Application version
	Date    string      // Build date
	Commit  string      // Git commit hash
	BuiltBy string      // Builder identifier
	Logger  *log.Logger // Global logger instance
)

// Asset represents a downloaded release asset with its name and MIME type
type Asset struct {
	Name     string // Filename of the downloaded asset
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
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands.
// It's the entry point for the `gh install` command.
var rootCmd = &cobra.Command{
	Use:   "install",
	Short: "gh installs binaries published on GitHub releases.",
	Long: `gh installs binaries published on GitHub releases.
Detects Operating System and Architecture to download and
install the appropriate binary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Expect the first argument to be in the format owner/repo[@version]
		a := args[0]

		// Parse the argument string into owner, repo, and optional version
		pa, err := utils.ParseArgs(a)
		if err != nil {
			fmt.Printf("error: %s", err)
		}

		// Create a background context for API calls
		ctx := context.Background()

		// Initialize the GitHub client
		client, err := ghclient.NewClient(ctx)
		if err != nil {
			// Log a fatal error and exit if client initialization fails
			log.Fatalf("Failed to initialize GitHub client: %v", err)
		}

		// Check and log the current GitHub API rate limit
		ghclient.CheckRateLimit(ctx, client)

		// Handle 'latest' version or empty version (defaults to latest)
		if pa.Version == "latest" || pa.Version == "" {
			// Get assets from the latest release
			assets := getLatestReleaseAssets(ctx, client, pa.Owner, pa.Repo)

			// Download the appropriate asset for current OS/architecture
			asset, err := downloadAsset(ctx, client, pa.Owner, pa.Repo, assets, http.DefaultClient)
			if err != nil {
				utils.Logger.Debugf("failed to download asset: %s", asset.Name)
				return fmt.Errorf("failed to download asset: %w", err)
			}
			utils.Logger.Debugf("doing the needful with %s ", asset.Name)
		} else {
			// Get assets from a specific release tag
			assets := getTaggedReleaseAssets(ctx, client, pa.Owner, pa.Repo, pa.Version)

			// Download the appropriate asset for current OS/architecture
			asset, err := downloadAsset(ctx, client, pa.Owner, pa.Repo, assets, http.DefaultClient)
			if err != nil {
				utils.Logger.Debugf("failed to download asset: %s", asset.Name)
				return fmt.Errorf("failed to download asset: %w", err)
			}
			utils.Logger.Debugf("doing the needful with %s ", asset.Name)
		}
		return nil
	},
}

// getLatestReleaseAssets retrieves all assets from the latest release of a repository.
//
// -ctx: The context for API calls, allows for cancellation/timeouts.
// -client: The initialized GitHub client for making API requests.
// -owner: The owner (user or organization) of the GitHub repository.
// -repo: The name of the GitHub repository.
//
// Returns: A slice of release assets, or nil if the API call fails.
func getLatestReleaseAssets(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
) (assets []*github.ReleaseAsset) {
	// Call the GitHub API to get the latest release
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		fmt.Printf("error: %s ", err)
		return nil
	}

	// Return all assets from the latest release
	return release.Assets
}

// getTaggedReleaseAssets retrieves all assets from a specific tagged release.
//
// -ctx: The context for API calls, allows for cancellation/timeouts.
// -client: The initialized GitHub client for making API requests.
// -owner: The owner (user or organization) of the GitHub repository.
// -repo: The name of the GitHub repository.
// -tag: The release tag to retrieve (e.g., "v1.0.0").
//
// Returns: A slice of release assets, or nil if the API call fails.
func getTaggedReleaseAssets(
	ctx context.Context,
	client *github.Client,
	owner, repo, tag string,
) (assets []*github.ReleaseAsset) {
	// Call the GitHub API to get a release by its tag name
	release, _, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		fmt.Printf("error: %s ", err)
		return nil
	}

	// Return all assets from the tagged release
	return release.Assets
}

// downloadAndSaveAsset downloads and saves a specific release asset to the local filesystem.
//
// -ctx: The context for API calls, allows for cancellation/timeouts.
// -client: The initialized GitHub client for making API requests.
// -owner: The owner (user or organization) of the GitHub repository.
// -repo: The name of the GitHub repository.
// -asset: The GitHub release asset to download.
// -httpClient: The HTTP client to use for the download.
//
// Returns:
//   - string: The name of the downloaded file
//   - string: The MIME content type of the downloaded file
//   - error: An error if the download or save operation fails
func downloadAndSaveAsset(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	asset *github.ReleaseAsset,
	httpClient *http.Client,
) (string, string, error) {
	// Validate that the asset has all required fields
	if asset == nil || asset.Name == nil || asset.ContentType == nil || asset.ID == nil ||
		asset.Size == nil {
		return "", "", errors.New("asset has missing information")
	}

	// Extract asset information from the GitHub API response
	assetName := *asset.Name
	assetID := *asset.ID
	assetSize := *asset.Size
	assetContentType := *asset.ContentType

	// Initiate the download through the GitHub API
	rc, redirectURL, err := client.Repositories.DownloadReleaseAsset(
		ctx,
		owner,
		repo,
		assetID,
		httpClient,
	)
	if err != nil {
		return "", "", fmt.Errorf("error initiating download: %w", err)
	}

	// Check if we received a valid response reader
	if rc == nil {
		if redirectURL == "" {
			return "", "", errors.New("both reader and redirect URL are nil")
		}
		return "", "", errors.New("download returned nil reader")
	}

	// Ensure reader is closed when function exits
	defer rc.Close() //nolint:errcheck

	// GitHub API should provide either a reader or a redirect URL, not both
	// Log a warning if we received both
	if redirectURL != "" {
		utils.Logger.Errorf(
			"Warning: Received non-nil reader AND redirect URL ('%s') for asset '%s'. Proceeding with download from reader.",
			redirectURL,
			assetName,
		)
	}

	// Save the asset data to a local file
	if err := saveAssetToFile(rc, assetName, int64(assetSize)); err != nil {
		return "", "", err
	}

	// Return the asset name and content type
	return assetName, assetContentType, nil
}

// saveAssetToFile saves asset data from a reader to a local file with progress display.
//
// -rc: The reader providing the asset data.
// -assetName: The name to use for the saved file.
// -assetSize: The expected size of the asset in bytes.
//
// Returns: An error if the file creation, data copying, or file closing fails.
func saveAssetToFile(rc io.ReadCloser, assetName string, assetSize int64) error {
	utils.Logger.Debugf("Starting download for '%s'...", assetName)

	// Create the output file using a safe path (base filename only)
	safeFile := filepath.Base(assetName)
	file, err := os.Create(safeFile) //nolint:gosec
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", assetName, err)
	}

	// Setup a progress bar to show download progress
	bar := progressbar.NewOptions64(
		assetSize,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(35), //nolint:mnd
		progressbar.OptionSetDescription(
			fmt.Sprintf("[cyan]Downloading %s...[reset]", assetName),
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
	_, copyErr := io.Copy(
		io.MultiWriter(file, bar),
		rc,
	)

	// Close the file *before* checking copyErr to ensure data is flushed
	closeErr := file.Close()

	// Handle errors during data copying
	if copyErr != nil {
		utils.Logger.Errorf("Error copying data for asset '%s': %v", assetName, copyErr)
		// Attempt to clean up partially written file
		_ = os.Remove(assetName)
		return fmt.Errorf("error copying data: %w", copyErr)
	}

	// Log but don't exit if there's an error closing the file
	if closeErr != nil {
		utils.Logger.Errorf(
			"Warning: Error closing file '%s' after download: %v",
			assetName,
			closeErr,
		)
	}

	utils.Logger.Printf("Successfully downloaded %s", assetName)
	return nil
}

// downloadAsset finds and downloads the most appropriate asset for the current OS/architecture.
//
// -ctx: The context for API calls, allows for cancellation/timeouts.
// -client: The initialized GitHub client for making API requests.
// -owner: The owner (user or organization) of the GitHub repository.
// -repo: The name of the GitHub repository.
// -assets: The list of available release assets to choose from.
// -httpClient: The HTTP client to use for the download.
//
// Returns:
//   - Asset: A struct containing information about the downloaded asset.
//   - error: An error if no matching asset is found or the download fails.
func downloadAsset(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	assets []*github.ReleaseAsset,
	httpClient *http.Client,
) (Asset, error) {
	// Use default HTTP client if none provided
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	utils.Logger.Debugf(
		"Attempting to find and download matching asset from %d available assets.",
		len(assets),
	)

	// Iterate through all assets to find one that matches current OS/architecture
	for _, asset := range assets {
		// Skip assets with missing information
		if asset == nil || asset.Name == nil {
			utils.Logger.Debug("Skipping asset with missing information.")
			continue
		}

		assetName := *asset.Name

		// Check if the asset name matches the current OS/architecture
		if !utils.MatchFile(assetName) {
			continue
		}

		utils.Logger.Debugf("Found matching asset: %s", assetName)

		// Download the matching asset
		downloadedName, downloadedMIMEType, err := downloadAndSaveAsset(
			ctx,
			client,
			owner,
			repo,
			asset,
			httpClient,
		)
		if err != nil {
			utils.Logger.Errorf(
				"Error downloading asset '%s': %v. Trying next asset.",
				assetName,
				err,
			)
			continue
		}

		// Return information about the successfully downloaded asset
		return Asset{Name: downloadedName, MIMEType: downloadedMIMEType}, nil
	}

	// If no matching asset was found or successfully downloaded
	return Asset{}, errors.New(
		"no matching asset found or successfully downloaded for the current OS/architecture",
	)
}

// Commented-out helper function for future implementation
// func isArchiveByGitHubContentType(contentType string) bool {
// 	// Common archive content types
// 	archiveTypes := map[string]bool{
// 		"application/zip":         true,
// 		"application/x-tar":       true,
// 		"application/gzip":        true,
// 		"application/x-gzip":      true,
// 		"application/x-xz":        true,
// 		"application/x-zstd":      true,
// 		"application/zstd":        true,
// 		"application/x-zstandard": true,
// 		"application/zstandard":   true,
// 	}
//
// 	return archiveTypes[contentType]
// }
