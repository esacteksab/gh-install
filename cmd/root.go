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
	"strings"

	"github.com/google/go-github/v71/github"
	kt "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

// ParsedArgs holds the parsed components of the argument string.
type ParsedArgs struct {
	Owner   string
	Repo    string
	Version string // Will be "latest" or a specific tag
}

type BinaryConfig struct {
	Key     string `koanf:"key"`
	Name    string `koanf:"name"`
	Version string `koanf:"version"`
}

type Config struct {
	Binaries map[string]BinaryConfig `koanf:"binaries"`
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
//
// Returns: Does not return a value, but exits the program with status code 1 if an error occurs.
func Execute() {
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
		pa, err := parseArgs(a)
		if err != nil {
			fmt.Printf("error: %s", err)
		}

		if pa.Version == "latest" || pa.Version == "" {
			assets := getLatestRelease(pa.Owner, pa.Repo)

			downloadAsset(pa.Owner, pa.Repo, assets)
		} else {
			assets := getTaggedRelease(pa.Owner, pa.Repo, pa.Version)

			downloadAsset(pa.Owner, pa.Repo, assets)
		}
	},
}

func LoadFromFile(path string) (Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(path), kt.Parser()); err != nil {
		return Config{}, err
	}

	config := Config{
		Binaries: make(map[string]BinaryConfig),
	}

	for _, key := range k.MapKeys("") {
		src := BinaryConfig{
			Key:     key,
			Name:    k.String(key + ".name"),
			Version: k.String(key + ".version"),
		}
		config.Binaries[key] = src
	}
	return config, nil
}

func getLatestRelease(owner, repo string) (assets []*github.ReleaseAsset) {
	client := github.NewClient(nil)
	ctx := context.Background()

	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		fmt.Printf("error: %s ", err)
		return nil
	}

	return release.Assets
}

func getTaggedRelease(owner, repo, tag string) (assets []*github.ReleaseAsset) {
	client := github.NewClient(nil)
	ctx := context.Background()

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

func downloadAsset(owner, repo string, assets []*github.ReleaseAsset) {
	client := github.NewClient(nil)
	ctx := context.Background()

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
			defer file.Close() //nolint:staticcheck,errcheck
			if err != nil {
				log.Fatalf("Unable to create file: %s", err)
			}

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

// parseArgs parses an argument string in the format owner/repo[@version].
// Supported formats:
// - owner/repo (version defaults to "latest")
// - owner/repo@latest
// - owner/repo@vX.Y.Z (or any other tag)
// Returns ParsedArgs and an error if the format is invalid.
// It also handles the "owner/repo@" case as an error.
func parseArgs(argString string) (ParsedArgs, error) {
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
