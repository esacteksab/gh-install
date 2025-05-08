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
	"strings"

	"github.com/adrg/xdg"
	"github.com/fatih/color"
	"github.com/google/go-github/v71/github"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/esacteksab/gh-install/ghclient"
	"github.com/esacteksab/gh-install/utils"
)

// Build information variables populated at build time
var (
	binNameFlag string // binNameFlag is the value from the --binName flag
	pathFlag    string // pathFlag is the value from the --path flag
	Version     string // Application version
	Date        string // Build date
	Commit      string // Git commit hash
	BuiltBy     string // Builder identifier
	green       = color.New(color.FgGreen).SprintFunc()
	red         = color.New(color.FgRed).SprintFunc()
	yellow      = color.New(color.FgYellow).SprintFunc()
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
func init() {
	// Creates initial logger with log level info
	utils.CreateLogger(false)
	rootCmd.Version = utils.BuildVersion(Version, Commit, Date, BuiltBy)
	rootCmd.SetVersionTemplate(`{{printf "Version %s" .Version}}`)

	// Path to save binary
	rootCmd.PersistentFlags().
		StringVarP(&pathFlag, "path", "p", "", "directory location to save binary")
	// Binary Name
	rootCmd.PersistentFlags().StringVarP(&binNameFlag, "binName", "b", "", "name to save binary as")
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	debugEnvVal := os.Getenv(ghInstallInitDebugEnv)
	initialVerbose, _ := strconv.ParseBool(debugEnvVal)
	utils.CreateLogger(initialVerbose)
	utils.Logger.Debugf(
		"Initial logger created in Execute(). Initial Verbose based on %s: %t",
		ghInstallInitDebugEnv,
		initialVerbose,
	)
	utils.GetOSArch()
	if err := rootCmd.Execute(); err != nil {
		utils.Logger.Errorf("error: %s", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "install owner/repo[@version]",
	SilenceUsage:  true,
	SilenceErrors: true,
	Short:         "gh installs binaries published on GitHub releases.",
	Long: `gh installs binaries published on GitHub releases.
Detects Operating System and Architecture to download and
install the appropriate binary. Includes checksum verification if available.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		a := args[0]
		pa, err := utils.ParseArgs(a)
		if err != nil {
			return fmt.Errorf("invalid argument: %w", err)
		}

		ctx := context.Background()
		client, err := ghclient.NewClient(ctx)
		if err != nil {
			utils.Logger.Errorf("Failed to initialize GitHub client: %v", err)
			return fmt.Errorf("failed to initialize GitHub client: %v", err)
		}
		ghclient.CheckRateLimit(ctx, client)

		var assets []*github.ReleaseAsset
		var releaseTag string

		if pa.Version == "latest" || pa.Version == "" {
			utils.Logger.Infof("Fetching assets for latest release of %s/%s", pa.Owner, pa.Repo)
			release, err := getLatestRelease(ctx, client, pa.Owner, pa.Repo)
			if err != nil {
				return fmt.Errorf("could not get latest release: %w", err)
			}
			assets = release.Assets
			releaseTag = release.GetTagName()
			utils.Logger.Infof("Latest release tag: %s", releaseTag)
		} else {
			utils.Logger.Infof("Fetching assets for release tag '%s' of %s/%s", pa.Version, pa.Owner, pa.Repo)
			release, err := getTaggedRelease(ctx, client, pa.Owner, pa.Repo, pa.Version)
			if err != nil {
				return fmt.Errorf("could not get release for tag '%s': %w", pa.Version, err)
			}
			assets = release.Assets
			releaseTag = release.GetTagName()
		}

		if len(assets) == 0 {
			return fmt.Errorf("no assets found for release '%s'", releaseTag)
		}

		downloadedAsset, err := findDownloadAndVerifyAsset(
			ctx,
			client,
			pa.Owner,
			pa.Repo,
			assets,
			http.DefaultClient,
		)
		if err != nil {
			return err
		}

		utils.Logger.Infof("Successfully downloaded and verified: %s", downloadedAsset.Name)
		utils.Logger.Infof("Asset saved to: %s", downloadedAsset.Path)
		utils.Logger.Debugf("Asset MIME Type: %s", downloadedAsset.MIMEType)
		utils.Logger.Info(">>> Next steps (unpacking, installation) are not yet implemented. <<<")
		return nil
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
		rateLimitInfo := ""
		if resp != nil {
			rateLimitInfo = resp.Rate.String()
		}
		return nil, fmt.Errorf(
			"failed to get latest release: %w (Rate Limit: %s)",
			err,
			rateLimitInfo,
		)
	}
	if release == nil {
		return nil, errors.New("received nil release object from GitHub API")
	}
	return release, nil
}

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
		rateLimitInfo := ""
		if resp != nil {
			rateLimitInfo = resp.Rate.String()
		}
		return nil, fmt.Errorf(
			"failed to get release by tag '%s': %w (Rate Limit: %s)",
			tag,
			err,
			rateLimitInfo,
		)
	}
	if release == nil {
		return nil, fmt.Errorf("received nil release object for tag '%s' from GitHub API", tag)
	}
	return release, nil
}

// downloadAndSaveAsset downloads a specific release asset and saves it to targetSavePath.
// Returns the path where the file was saved (which is targetSavePath on success) and any error.
func downloadAndSaveAsset(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	asset *github.ReleaseAsset,
	httpClient *http.Client,
	targetSavePath string,
) (filePath string, err error) {
	if asset == nil || asset.Name == nil || asset.ID == nil || asset.Size == nil {
		return "", errors.New("asset has missing information (name, id, or size)")
	}

	assetName := *asset.Name
	assetID := *asset.ID
	assetSize := *asset.Size

	utils.Logger.Debugf(
		"Initiating download for asset: %s (ID: %d, Size: %d) to target path: %s",
		assetName,
		assetID,
		assetSize,
		targetSavePath,
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
		if redirectURL != "" {
			utils.Logger.Warnf(
				"Download for '%s' resulted in a redirect URL (%s) but no reader.",
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

	if redirectURL != "" {
		utils.Logger.Warnf(
			"Received both a reader and a redirect URL ('%s') for asset '%s'. Proceeding with download.",
			redirectURL,
			assetName,
		)
	}

	// Use the provided targetSavePath to save the file
	err = saveAssetToFile(rc, targetSavePath, assetName, int64(assetSize))
	if err != nil {
		// Error already contains context from saveAssetToFile
		return targetSavePath, err // Return targetSavePath even on error for potential cleanup
	}

	// Return the path where the file was saved
	return targetSavePath, nil
}

// saveAssetToFile saves asset data from a reader to a local file with progress display.
// localPath is the exact path where the file should be created.
// displayName is the original asset name for the progress bar.
func saveAssetToFile(rc io.ReadCloser, localPath, displayName string, assetSize int64) error {
	utils.Logger.Debugf("Saving asset '%s' to specific local path '%s'", displayName, localPath)

	// Create the output file at the specified localPath
	file, err := os.Create(localPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("error creating file '%s': %w", localPath, err)
	}
	var fileClosed bool
	defer func() {
		if !fileClosed {
			file.Close() //nolint:errcheck,gosec
		}
	}()

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

	_, copyErr := io.Copy(io.MultiWriter(file, bar), rc)
	closeErr := file.Close()
	fileClosed = true

	if copyErr != nil {
		utils.Logger.Errorf(
			"Error during download/copy for '%s' to '%s': %v",
			displayName,
			localPath,
			copyErr,
		)
		_ = os.Remove(localPath) // Attempt cleanup on copy error
		return fmt.Errorf("error saving data for '%s' to '%s': %w", displayName, localPath, copyErr)
	}
	if closeErr != nil {
		utils.Logger.Errorf("Error closing file '%s' after download: %v", localPath, closeErr)
		// Do not return error here if copy was successful, but log it.
	}

	utils.Logger.Printf(green("✔")+" Successfully downloaded %s to %s", displayName, localPath)
	return nil
}

// need to address gocyclo
// funlen 52 > 50 -- maybe not an issue
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
				utils.Logger.Warnf("Found multiple checksum files. Using '%s', ignoring '%s'.", *checksumAssetToDownload.Name, assetName)
			}
			continue
		}
		if utils.MatchFile(assetName) {
			if mainAssetToDownload == nil {
				utils.Logger.Debugf("Found potential main asset: %s", assetName)
				mainAssetToDownload = asset
			} else {
				utils.Logger.Warnf("Found multiple matching assets. Using '%s', ignoring '%s'.", *mainAssetToDownload.Name, assetName)
			}
		}
	}

	if mainAssetToDownload == nil {
		utils.Logger.Error("No asset matching OS/Arch found.")
		return Asset{}, errors.New("no suitable asset found for download")
	}

	utils.Logger.Infof("Selected main asset for download: %s", *mainAssetToDownload.Name)
	if checksumAssetToDownload != nil {
		utils.Logger.Infof("Selected checksum file: %s", *checksumAssetToDownload.Name)
	} else {
		utils.Logger.Warn(yellow("No checksum file found. Proceeding without verification."))
	}

	// Determine Save Path for Main Asset
	var finalMainAssetSaveName string
	if binNameFlag != "" { // User specified --binName
		finalMainAssetSaveName = binNameFlag
	} else {
		// Default name: first part of original asset name, split by '_'
		sbn := strings.Split(*mainAssetToDownload.Name, "_")
		finalMainAssetSaveName = sbn[0]
		// Consider if asset name is simple like "mytool" or "mytool.exe"
		if len(sbn) == 1 {
			finalMainAssetSaveName = *mainAssetToDownload.Name // use full original name if no underscore
			// If it's an archive and user didn't specify binName, keep original name to preserve extension
			// If it's not an archive, and has an extension, e.g. mytool.exe, this is fine.
			// This part could be refined based on whether it's an archive or raw binary.
			// For now, if sbn[0] is the full name, use it. If it's part of a complex name, use sbn[0].
			// A simple robust default if no binNameFlag: use the original asset name.
			// Let's refine the default:
			// If it's likely a raw binary (not archive, not installer usually ending in .exe, .dmg etc)
			// then sbn[0] might be good. Otherwise, *mainAssetToDownload.Name is safer.
			// For now, sticking to your provided logic:
			// finalMainAssetSaveName = sbn[0]; is already set.
		}
	}

	var targetMainAssetDir string
	switch {
	case pathFlag != "" && pathFlag != ".": // User specified --path directory
		targetMainAssetDir = filepath.Clean(pathFlag)
	case pathFlag == ".": // User specified current directory
		targetMainAssetDir = "."
	default: // Default to XDG Bin Home
		targetMainAssetDir = xdg.BinHome
	}

	// Ensure the target directory exists (unless it's current dir)
	if targetMainAssetDir != "." {
		if err := os.MkdirAll(targetMainAssetDir, 0o750); err != nil { //nolint:mnd
			return Asset{}, fmt.Errorf(
				"failed to create target directory '%s': %w",
				targetMainAssetDir,
				err,
			)
		}
	}
	targetMainAssetSavePath := filepath.Join(targetMainAssetDir, finalMainAssetSaveName)
	utils.Logger.Debugf(
		"Main asset ('%s') will be saved as: %s",
		*mainAssetToDownload.Name,
		targetMainAssetSavePath,
	)

	// Download Main Asset
	downloadedMainAssetActualPath, err := downloadAndSaveAsset(
		ctx, client, owner, repo, mainAssetToDownload, httpClient, targetMainAssetSavePath,
	)
	if err != nil {
		// downloadAndSaveAsset now includes targetMainAssetSavePath in its error reporting if relevant
		return Asset{}, fmt.Errorf(
			"failed to download main asset '%s': %w",
			*mainAssetToDownload.Name,
			err,
		)
	}
	// downloadedMainAssetActualPath should be == targetMainAssetSavePath on success

	// Download Checksum File and Verify (if found)
	if checksumAssetToDownload != nil {
		// Checksum file is always downloaded to the current directory with its original name
		targetChecksumAssetSavePath := filepath.Clean(filepath.Base(*checksumAssetToDownload.Name))
		utils.Logger.Debugf(
			"Checksum asset ('%s') will be saved as: %s",
			*checksumAssetToDownload.Name,
			targetChecksumAssetSavePath,
		)

		actualChecksumAssetPath, checksumErr := downloadAndSaveAsset(
			ctx,
			client,
			owner,
			repo,
			checksumAssetToDownload,
			httpClient,
			targetChecksumAssetSavePath,
		)
		if checksumErr != nil {
			utils.Logger.Errorf(
				red(
					"Failed to download checksum file '%s': %v. Checksum verification will be SKIPPED.",
				),
				*checksumAssetToDownload.Name,
				checksumErr,
			)
			utils.Logger.Warnf(
				yellow("Integrity of '%s' (at %s) is NOT confirmed."),
				*mainAssetToDownload.Name, downloadedMainAssetActualPath,
			)
			// Proceed without verification in this case
		} else {
			// Pass the actual path of the (potentially renamed/relocated) main asset
			// and its original name for checksum lookup
			verifyErr := verifyAssetChecksum(downloadedMainAssetActualPath, *mainAssetToDownload.Name, actualChecksumAssetPath)
			if verifyErr != nil {
				// Verification failed. verifyAssetChecksum handles cleanup of downloadedMainAssetActualPath.
				return Asset{}, verifyErr // verifyErr already contains context
			}
			// Verification successful, checksum file (actualChecksumAssetPath) removed by verifyAssetChecksum.
		}
	}

	return Asset{
		Name:     *mainAssetToDownload.Name,
		Path:     downloadedMainAssetActualPath,
		MIMEType: *mainAssetToDownload.ContentType,
	}, nil
}

func verifyAssetChecksum(mainAssetDiskPath, mainAssetOriginalName, checksumAssetPath string) error {
	utils.Logger.Info("Verifying checksum...")

	expectedChecksum, err := utils.ParseChecksumFile(checksumAssetPath, mainAssetOriginalName)
	if err != nil {
		utils.Logger.Errorf(
			"Failed to find/parse checksum for '%s' in '%s': %v",
			mainAssetOriginalName, checksumAssetPath, err,
		)
		_ = os.Remove(mainAssetDiskPath)
		_ = os.Remove(checksumAssetPath)
		return fmt.Errorf(
			"checksum verification failed for '%s': could not find entry in checksum file '%s'",
			mainAssetOriginalName, filepath.Base(checksumAssetPath),
		)
	}

	isValid, algo, err := utils.VerifyChecksum(
		mainAssetDiskPath,
		checksumAssetPath,
		utils.DefaultAlgorithmForGenericChecksums,
	)
	if err != nil {
		log.Fatalf("Verification error: %v (Algorithm attempted: %s)", err, algo)
	}
	if isValid {
		log.Printf("File '%s' is valid using %s!", mainAssetDiskPath, algo)
	} else {
		log.Printf("File '%s' IS INVALID using %s!", mainAssetDiskPath, algo) // This branch means checksums didn't match
	}

	actualChecksum, err := utils.HashFile(mainAssetDiskPath, algo)
	if err != nil {
		utils.Logger.Errorf(
			"Failed to calculate checksum for downloaded file '%s' (original name '%s'): %v",
			mainAssetDiskPath, mainAssetOriginalName, err,
		)
		_ = os.Remove(mainAssetDiskPath)
		_ = os.Remove(checksumAssetPath)
		return fmt.Errorf(
			"checksum verification failed: could not hash downloaded file '%s' (original: %s)",
			mainAssetDiskPath, mainAssetOriginalName,
		)
	}

	if !strings.EqualFold(expectedChecksum, actualChecksum) {
		utils.Logger.Errorf(
			"CHECKSUM MISMATCH for %s (file: %s)!",
			mainAssetOriginalName,
			mainAssetDiskPath,
		)
		utils.Logger.Errorf("  Expected: %s", expectedChecksum)
		utils.Logger.Errorf("  Actual:   %s", actualChecksum)
		_ = os.Remove(mainAssetDiskPath)
		_ = os.Remove(checksumAssetPath)
		return errors.New(red("checksum mismatch - downloaded file is corrupt or incorrect"))
	}

	utils.Logger.Print(green("✔") + " Checksum verified successfully.")
	err = os.Remove(checksumAssetPath)
	if err != nil {
		utils.Logger.Warnf(
			"Could not remove checksum file '%s' after verification: %v",
			checksumAssetPath, err,
		)
	} else {
		utils.Logger.Debugf("Removed checksum file: %s", checksumAssetPath)
	}
	return nil
}
