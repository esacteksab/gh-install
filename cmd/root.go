// SPDX-License-Identifier: MIT
package cmd

import (
	"context"
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

var Logger *log.Logger

// --- Environment variable for init-phase debugging ---
const ghInstallInitDebugEnv = "GH_INSTALL_INIT_DEBUG" // Or your preferred name

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
//
// Returns: Does not return a value, but exits the program with status code 1 if an error occurs.
func Execute() {
	// Initial Logger -- InfoLevel
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
	Run: func(cmd *cobra.Command, args []string) {
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
			assets := getLatestRelease(ctx, client, pa.Owner, pa.Repo)

			downloadAsset(ctx, client, pa.Owner, pa.Repo, assets)
		} else {
			assets := getTaggedRelease(ctx, client, pa.Owner, pa.Repo, pa.Version)

			downloadAsset(ctx, client, pa.Owner, pa.Repo, assets)
		}
	},
}

func getLatestRelease(
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

func getTaggedRelease(
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

func getFile(file string) bool {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// OS / Arch
	oa := os + "-" + arch

	// []byte file
	foa := ([]byte)(file)

	switch true {
	case regexp.MustCompile(oa).Match(foa):
		return true
	default:
		return false
	}
}

func downloadAsset(
	ctx context.Context,
	client *github.Client,
	owner, repo string,
	assets []*github.ReleaseAsset,
) {
	for _, a := range assets {
		if getFile(*a.Name) {
			ID := *a.ID
			asset, rurl, err := client.Repositories.DownloadReleaseAsset(ctx,
				owner, repo, ID, http.DefaultClient)
			if err != nil {
				log.Fatal(err)
			}

			if rurl != "" {
				fmt.Println(rurl)
			}

			file, err := os.Create(*a.Name)
			if err != nil {
				log.Fatalf("Unable to create file: %s", err)
			}
			defer file.Close() //nolint:errcheck

			bar := progressbar.NewOptions(*a.Size,
				progressbar.OptionShowBytes(true),
				progressbar.OptionSetDescription("Downloading..."),
				progressbar.OptionClearOnFinish())

			_, err = io.Copy(io.MultiWriter(file, bar), asset)
			if err != nil {
				log.Fatalf("Error writing file: %s", err)
			}

			_, err = os.Stat(file.Name())
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("Failed to download %s: %s", *a.Name, err)
				}
			}
			fmt.Printf("Successfully downloaded %s\n", *a.Name)
		}
	}
}
