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
	"regexp"
	"runtime"
	"strconv"

	"github.com/google/go-github/v71/github"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/esacteksab/gh-install/ghclient"
	"github.com/esacteksab/gh-install/utils"
)

var (
	Version       string // Application version
	Date          string // Build date
	Commit        string // Git commit hash
	BuiltBy       string // Builder identifier
	Logger        *log.Logger
	osArchRegexes []*regexp.Regexp
)

type Asset struct {
	Name     string
	MIMEType string
}

// --- Environment variable for init-phase debugging ---
const ghInstallInitDebugEnv = "GH_INSTALL_INIT_DEBUG" // Or your preferred name

func init() {
	// BuildVersion utility formats the version string.
	rootCmd.Version = utils.BuildVersion(Version, Commit, Date, BuiltBy)
	// SetVersionTemplate customizes how the version is printed.
	rootCmd.SetVersionTemplate(`{{printf "Version %s" .Version}}`)

	os := runtime.GOOS
	arch := runtime.GOARCH

	// Escape special regex characters in OS and arch
	quotedOS := regexp.QuoteMeta(os)

	// Create architecture mappings for common variants
	var archPatterns []string

	// Add the default Go architecture name
	archPatterns = append(archPatterns, regexp.QuoteMeta(arch))

	// Add common alternative architecture names
	switch arch {
	case "amd64":
		archPatterns = append(archPatterns, "x86_64")
	case "386":
		archPatterns = append(archPatterns, "i386")
	}

	// Create all combinations of patterns
	var patterns []string
	for _, archPattern := range archPatterns {
		// Create and compile patterns for all three separator types
		patterns = append(patterns, fmt.Sprintf("(?i)%s-%s", quotedOS, archPattern))
		patterns = append(patterns, fmt.Sprintf("(?i)%s/%s", quotedOS, archPattern))
		patterns = append(patterns, fmt.Sprintf("(?i)%s_%s", quotedOS, archPattern))

		// Add patterns that might have the OS name at the beginning or end of the filename
		patterns = append(patterns, fmt.Sprintf("(?i)%s.*%s", quotedOS, archPattern))
		patterns = append(patterns, fmt.Sprintf("(?i)%s.*%s", archPattern, quotedOS))
	}

	// Pre-compile all the patterns
	osArchRegexes = make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		osArchRegexes[i] = regexp.MustCompile(pattern)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
//
// Returns: Does not return a value, but exits the program with status code 1 if an error occurs.
func Execute() {
	// Initial Logger --
	// createLogger(false)
	// --- Check ENV VAR for Initial Verbosity ---
	debugEnvVal := os.Getenv(ghInstallInitDebugEnv)
	// Parse bool allows "true", "TRUE", "True", "1"
	initialVerbose, _ := strconv.ParseBool(debugEnvVal)
	// If parsing fails (e.g., empty string), initialVerbose remains false

	// --- Create INITIAL logger based on ENV VAR ---
	utils.CreateLogger(initialVerbose) // Initia/Configlize with level based on debug env var
	// This log will NOW appear if GH_TP_INIT_DEBUG=true
	utils.Logger.Debugf(
		"Initial logger created in Execute(). Initial Verbose based on %s: %t",
		ghInstallInitDebugEnv,
		initialVerbose,
	)
	// Execute the root command. If an error occurs, print it to stderr and exit.
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "install",
	Short: "gh installs binaries published on GitHub releases.",
	Long: `gh installs binaries published on GitHub releases.
Detects Operating System and Architecture to download and
install the appropriate binary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		a := args[0]
		pa, err := utils.ParseArgs(a)
		if err != nil {
			fmt.Printf("error: %s", err)
		}

		// context.Background() is the default context, suitable for the top-level command.
		ctx := context.Background()

		// Initialize the GitHub client using the dedicated package.
		client, err := ghclient.NewClient(ctx)
		if err != nil {
			// Log a fatal error and exit if the client cannot be initialized.
			log.Fatalf("Failed to initialize GitHub client: %v", err)
		}

		// Check the current GitHub API rate limit. This is helpful for debugging potential rate limit issues.
		ghclient.CheckRateLimit(ctx, client)

		if pa.Version == "latest" || pa.Version == "" {
			assets := getLatestReleaseAssets(ctx, client, pa.Owner, pa.Repo)

			asset, err := downloadAsset(ctx, client, pa.Owner, pa.Repo, assets)
			if err != nil {
				utils.Logger.Debugf("failed to download asset: %s", asset.Name)
				return fmt.Errorf("failed to download asset: %w", err)
			}
			utils.Logger.Debugf("doing the needful with %s ", asset.Name)
		} else {
			assets := getTaggedReleaseAssets(ctx, client, pa.Owner, pa.Repo, pa.Version)

			asset, err := downloadAsset(ctx, client, pa.Owner, pa.Repo, assets)
			if err != nil {
				utils.Logger.Debugf("failed to download asset: %s", asset.Name)
				return fmt.Errorf("failed to download asset: %w", err)
			}
			utils.Logger.Debugf("doing the needful with %s ", asset.Name)
		}
		return nil
	},
}

func getLatestReleaseAssets(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
) (assets []*github.ReleaseAsset) {
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		fmt.Printf("error: %s ", err)
		return nil
	}

	return release.Assets
}

func getTaggedReleaseAssets(
	ctx context.Context,
	client *github.Client,
	owner, repo, tag string,
) (assets []*github.ReleaseAsset) {
	release, _, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		fmt.Printf("error: %s ", err)
		return nil
	}

	return release.Assets
}

// matchFile checks if the file name matches the current OS/arch pattern
func matchFile(file string) bool {
	// Convert the file string to []byte
	foa := []byte(file)

	// Check if the file matches any of the pre-compiled patterns
	for i, re := range osArchRegexes {
		if re.Match(foa) {
			utils.Logger.Debugf("File '%s' matched pattern %d: %s", file, i, re.String())
			return true
		}
	}
	utils.Logger.Debugf("File '%s', did not match any OS/arch pattern", file)
	return false
}

func downloadAsset(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	assets []*github.ReleaseAsset,
) (Asset, error) {
	// assetName
	var an string
	var ct string
	downloadSuccess := false

	utils.Logger.Debugf("assets: %s\n", assets)
	for _, a := range assets {
		if matchFile(*a.Name) {
			an = *a.Name
			utils.Logger.Debugf("Asset Name: %s", an)
			ct = *a.ContentType
			utils.Logger.Debugf("Asset Content Type: %s", ct)
			ID := *a.ID
			utils.Logger.Debugf("Asset ID: %d", ID)
			asset, rurl, err := client.Repositories.DownloadReleaseAsset(ctx,
				owner, repo, ID, http.DefaultClient)
			if err != nil {
				log.Fatal(err)
			}

			utils.Logger.Debugf("Asset Name: %s", *a.Name)

			if rurl != "" {
				utils.Logger.Debugf("Redirect URL: %s", rurl)
			}

			file, err := os.Create(*a.Name)
			if err != nil {
				log.Fatalf("Unable to create file: %s", err)
			}
			defer file.Close() //nolint:errcheck

			bar := progressbar.NewOptions(*a.Size,
				progressbar.OptionShowBytes(true),
				progressbar.OptionSetWidth(35), //nolint:mnd
				progressbar.OptionSetDescription("Downloading..."),
				progressbar.OptionClearOnFinish())

			_, err = io.Copy(io.MultiWriter(file, bar), asset)
			if err != nil {
				return Asset{}, fmt.Errorf("writing file: %s", err)
			}

			_, err = os.Stat(file.Name())
			if err != nil {
				if os.IsNotExist(err) {
					return Asset{}, fmt.Errorf("failed to download %s: %s", *a.Name, err)
				}
			}
			fmt.Printf("Successfully downloaded %s\n", *a.Name)
			downloadSuccess = true
			break
		}
	}

	if !downloadSuccess {
		return Asset{}, errors.New("no matching asset found for current OS/architecture")
	}

	return Asset{Name: an, MIMEType: ct}, nil
}

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
