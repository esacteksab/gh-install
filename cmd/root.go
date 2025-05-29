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
	"slices"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	"github.com/fatih/color"
	"github.com/google/go-github/v72/github"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/esacteksab/gh-install/ghclient"
	"github.com/esacteksab/gh-install/utils"
)

// Build information variables populated at build time
var (
	binNameFlag string // binNameFlag is the value from the --binName flag
	pathFlag    string // pathFlag is the value from the --path flag
	shaFlag     string // shaFlag is the value from the --sha flag
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

	supportedAlgos := utils.ListSupportedAlgorithms()
	algoListString := strings.Join(supportedAlgos, ", ")
	usageMessage := fmt.Sprintf(
		"SHA algorithm to use for checksum verification. Valid algorithms are: %s.",
		algoListString,
	)

	// Path to save binary
	rootCmd.PersistentFlags().
		StringVarP(&pathFlag,
			"path",
			"p",
			"",
			"directory location to save binary. Default: $XDG_BIN_HOME")
	// Binary Name
	rootCmd.PersistentFlags().StringVarP(
		&binNameFlag,
		"binName",
		"b",
		"",
		"name to save binary as")
	rootCmd.PersistentFlags().
		StringVarP(
			&shaFlag,
			"sha",
			"s",
			"",
			usageMessage)
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

		// Right now all we do is check, but don't do any rate-limiting or
		// retrying or backing off if the limit is met or exceeded.
		// Probably want to do something about this, but low priority given the
		// existing limit, there would be _a lot_ of calls (actions, workflows)
		// to exceed those limits
		limitType := ghclient.CheckRateLimit(ctx, client)
		utils.LogRateLimitStatus(limitType)

		var assets []*github.ReleaseAsset
		var releaseTag string

		if pa.Version == "latest" || pa.Version == "" {
			utils.Logger.Printf("Fetching assets for latest release of %s/%s", pa.Owner, pa.Repo)
			release, err := getLatestRelease(ctx, client, pa.Owner, pa.Repo)
			if err != nil {
				return fmt.Errorf("could not get latest release: %w", err)
			}
			assets = release.Assets
			// utils.Logger.Debugf("Release Assets: %s", assets)
			releaseTag = release.GetTagName()
			utils.Logger.Printf("Latest release tag: %s", releaseTag)
		} else {
			utils.Logger.Printf("Fetching assets for release tag '%s' of %s/%s", pa.Version, pa.Owner, pa.Repo)
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

		utils.Logger.Debugf("Successfully downloaded and verified: %s", downloadedAsset.Name)
		utils.Logger.Debugf("Asset saved to: %s", downloadedAsset.Path)
		utils.Logger.Debugf("Asset MIME Type: %s", downloadedAsset.MIMEType)
		// get extension of asset (if it exists)
		ext := utils.GetExtension(downloadedAsset.Name)
		utils.Logger.Debugf("Asset extension: %s", ext)

		exts := utils.ListSupportedSystemPackages()

		// if an extension exists, its assumed to be a system package, not a
		// binary and we don't need to chmod a system package
		if slices.Contains(exts, ext) {
			utils.Logger.Debugf("System Extension %s matched", ext)
			utils.Logger.Debugf("NOT chmod'ing %s", downloadedAsset.Name)
		} else {
			utils.Logger.Debug("No matching system extension found")
			utils.Logger.Debugf("chmod'ing %s", downloadedAsset.Name)
			utils.ChmodFile(downloadedAsset.Path)
		}
		utils.Logger.Debug(">>> Next steps (unpacking, installation) are not yet implemented. <<<")
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
	if !utils.IsChecksumFile(localPath) {
		utils.Logger.Debugf(green("✔")+" Successfully downloaded %s to %s", displayName, localPath)
		utils.Logger.Print(green("✔") + " Successfully downloaded")
	}
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
			utils.Logger.Debugf("Found potential main asset: %s", assetName)

			// This is a file with an extension (like .deb, .rpm, .apk)
			ext := filepath.Ext(assetName)
			osExt := utils.DetectOS()
			if ext != "" {
				utils.Logger.Debugf("Operating System Family: %s", osExt)

				if strings.Contains(assetName, osExt) {
					// This matches our OS package type - prefer this over any previous selection
					if mainAssetToDownload != nil {
						utils.Logger.Debugf("Replacing '%s' with OS-matching asset '%s'",
							*mainAssetToDownload.Name, assetName)
					}
					mainAssetToDownload = asset
					continue // keep this as our preferred choice but continue scanning
				}
			}

			// If we haven't found an OS-matching package yet, use this as fallback
			if mainAssetToDownload == nil {
				mainAssetToDownload = asset
			} else if !strings.Contains(*mainAssetToDownload.Name, osExt) {
				// it also doesn't match OS if we get here
				utils.Logger.Warnf("Found multiple non-OS-matching assets. Using '%s', ignoring '%s'.",
					*mainAssetToDownload.Name, assetName)
			} else {
				// already have an OS-matching asset, ignore this one
				utils.Logger.Warnf("Found multiple matching assets. Using '%s', ignoring '%s'.",
					*mainAssetToDownload.Name, assetName)
			}
		}
	}

	if mainAssetToDownload == nil {
		utils.Logger.Error("No asset matching OS/Arch found.")
		return Asset{}, errors.New("no suitable asset found for download")
	}

	utils.Logger.Debugf("Selected main asset for download: %s", *mainAssetToDownload.Name)
	if checksumAssetToDownload != nil {
		utils.Logger.Debugf("Selected checksum file: %s", *checksumAssetToDownload.Name)
	} else {
		utils.Logger.Warn(yellow("No checksum file found. Proceeding without verification."))
	}

	// Determine Save Path for Main Asset
	var finalMainAssetSaveName string
	// Check if the asset has a file extension
	ext := utils.GetExtension(*mainAssetToDownload.Name)
	if binNameFlag != "" { // User specified --binName
		finalMainAssetSaveName = binNameFlag
	} else {
		// final main asset name (fman)
		fman := utils.ParseBinaryName(*mainAssetToDownload.Name)
		finalMainAssetSaveName = fman
	}

	var targetMainAssetDir string
	switch {
	case pathFlag != "" && pathFlag != ".": // User specified --path directory
		targetMainAssetDir = filepath.Clean(pathFlag)
	case pathFlag == ".": // current working directory
		targetMainAssetDir = "."
	default:
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

	var targetMainAssetSavePath string
	// temp dir for system packages and eventually archives/compressed assets
	td, err := os.MkdirTemp("", "")
	if err != nil {
		return Asset{}, fmt.Errorf("failed to create temp dir: %s", err)
	}

	if ext != "" {
		// System package - save to temp directory with original filename
		targetMainAssetSavePath = filepath.Join(td, finalMainAssetSaveName)
		utils.Logger.Debugf(
			"Main asset ('%s') will be saved to temp dir as: %s",
			*mainAssetToDownload.Name,
			targetMainAssetSavePath,
		)
	} else {
		// Binary executable - save to target directory
		targetMainAssetSavePath = filepath.Join(targetMainAssetDir, finalMainAssetSaveName)
		utils.Logger.Debugf(
			"Main asset ('%s') will be saved as: %s",
			*mainAssetToDownload.Name,
			targetMainAssetSavePath,
		)
	}
	// download main asset
	downloadedMainAssetActualPath, err := downloadAndSaveAsset(
		ctx, client, owner, repo, mainAssetToDownload, httpClient,
		targetMainAssetSavePath)
	if err != nil {
		return Asset{}, fmt.Errorf(
			"failed to download main asset '%s': %w",
			*mainAssetToDownload.Name,
			err,
		)
	}
	// download Checksum File and Verify (if found)
	if checksumAssetToDownload != nil {
		// checksum file is always downloaded to the current directory with its original name
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
			verifyErr := verifyAssetChecksum(downloadedMainAssetActualPath, *mainAssetToDownload.Name, actualChecksumAssetPath, shaFlag)
			if verifyErr != nil {
				return Asset{}, verifyErr // Verification failed.
			}
			// Verification successful, checksum file (actualChecksumAssetPath) removed by verifyAssetChecksum.
			_ = os.Remove(actualChecksumAssetPath)
		}
	}

	return Asset{
		Name:     *mainAssetToDownload.Name,
		Path:     downloadedMainAssetActualPath,
		MIMEType: *mainAssetToDownload.ContentType,
	}, nil
}

func verifyAssetChecksum(
	mainAssetDiskPath, mainAssetOriginalName, checksumAssetPath, shaFlag string,
) error {
	utils.Logger.Debug("Verifying checksum...")
	var algoToUse string
	var expectedChecksum string
	var err error

	if shaFlag != "" {
		// User specified an algorithm
		algoToUse = shaFlag
		utils.Logger.Debugf("Using specified algorithm '%s' from --sha flag.", algoToUse)
		expectedChecksum, err = utils.ParseChecksumFile(checksumAssetPath, mainAssetOriginalName)
		if err != nil {
			return fmt.Errorf(
				"failed to parse checksum file '%s' for target '%s' (using --sha=%s): %w",
				checksumAssetPath,
				mainAssetOriginalName,
				shaFlag,
				err,
			)
		}
	} else {
		// Determine algorithm and get expected checksum via VerifyChecksum's parsing logic
		// We call VerifyChecksum to determine the algorithm and get the expected sum.
		// Temporarily, let's just determine the algorithm first
		var determinedAlgoFromExtOrGeneric string
		algoFromExt, found := utils.GetAlgorithmFromFilename(checksumAssetPath)
		if found {
			determinedAlgoFromExtOrGeneric = algoFromExt
			utils.Logger.Debugf("Using algorithm '%s' derived from checksum file extension: %s", determinedAlgoFromExtOrGeneric, checksumAssetPath)
		} else {
			determinedAlgoFromExtOrGeneric = utils.DefaultAlgorithmForGenericChecksums
			utils.Logger.Debugf("Checksum file '%s' has no algorithm extension. Using default/hint: '%s'", checksumAssetPath, determinedAlgoFromExtOrGeneric)
		}
		// Ensure determined algo is supported
		if _, err := utils.GetHasher(determinedAlgoFromExtOrGeneric); err != nil {
			return fmt.Errorf("algorithm '%s' (derived or default) is not supported: %w", determinedAlgoFromExtOrGeneric, err)
		}
		algoToUse = determinedAlgoFromExtOrGeneric

		expectedChecksum, err = utils.ParseChecksumFile(checksumAssetPath, mainAssetOriginalName)
		if err != nil {
			return fmt.Errorf("failed to parse checksum file '%s' for target '%s' (algorithm hint: %s): %w",
				checksumAssetPath, mainAssetOriginalName, algoToUse, err)
		}
	}

	utils.Logger.Debugf(
		"Calculating %s checksum for local asset: %s",
		strings.ToUpper(algoToUse),
		mainAssetDiskPath,
	)
	actualChecksum, err := utils.HashFile(mainAssetDiskPath, algoToUse)
	if err != nil {
		return fmt.Errorf("failed to calculate actual checksum for asset '%s' using %s: %w",
			mainAssetDiskPath, algoToUse, err)
	}

	if !strings.EqualFold(expectedChecksum, actualChecksum) {
		return fmt.Errorf(
			"checksum mismatch for asset '%s' (original name '%s') using algorithm '%s': expected '%s', got '%s'",
			mainAssetDiskPath,
			mainAssetOriginalName,
			algoToUse,
			expectedChecksum,
			actualChecksum,
		)
	}

	utils.Logger.Debugf(
		green("✔")+" Checksum VALID for '%s' (original name: '%s') using algorithm %s.",
		mainAssetDiskPath,
		mainAssetOriginalName,
		algoToUse,
	)
	utils.Logger.Print(green("✔") + " Checksum verified!")
	return nil
}
