// SPDX-License-Identifier: MIT

package ghclient_test

import (
	"bytes"
	"context"

	// "io" // No longer strictly needed if not using a variable for os.Stderr
	"os"
	"testing"
	"time"

	"github.com/google/go-github/v71/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/esacteksab/gh-install/ghclient"
	"github.com/esacteksab/gh-install/utils"
)

// Helper function to capture log output from utils.Logger (charmbracelet/log)
func captureLogOutput(fn func()) string {
	var buf bytes.Buffer

	if utils.Logger == nil {
		utils.CreateLogger(true) // Fallback initialization
	}

	// charmbracelet/log.Logger doesn't have direct getters for its current output.
	// We will set the output to our buffer for capture.
	// The original output writer is known to be os.Stderr from utils.CreateLogger.

	// Save the current configuration for restoration if possible, or restore to known defaults.
	// For charmbracelet/log, we'll restore to the typical verbose state.
	// These are the settings typically set by utils.CreateLogger(true)
	restoreReportTimestamp := true
	restoreReportCaller := true
	// The TimeFormat is set by CreateLogger based on verbose,
	// and SetReportTimestamp(true) will use the existing format.

	// Temporarily change logger settings for capture
	utils.Logger.SetOutput(&buf)
	utils.Logger.SetReportTimestamp(false) // Disable for predictable test output
	utils.Logger.SetReportCaller(false)    // Disable for predictable test output

	defer func() {
		// Restore logger settings
		utils.Logger.SetOutput(os.Stderr) // utils.CreateLogger always uses os.Stderr
		utils.Logger.SetReportTimestamp(restoreReportTimestamp)
		utils.Logger.SetReportCaller(restoreReportCaller)
		// If utils.CreateLogger was called with true, it sets a specific time format.
		// SetReportTimestamp(true) should reuse it. If CreateLogger(false) was called,
		// then timeFormat was "", and SetReportTimestamp(true) alone might not bring back
		// a specific format if one was desired. However, for test log capturing,
		// this restoration is generally sufficient.
		// If a very specific TimeFormat needs restoration, and CreateLogger's state is complex,
		// one might need to call utils.CreateLogger(true) again in the defer,
		// but that might have other side effects if CreateLogger does more than just set these.
		// For now, this simpler restoration is cleaner.
	}()

	fn() // Execute the function that logs
	return buf.String()
}

func TestNewClient_WithToken(t *testing.T) {
	utils.CreateLogger(true)

	tempCacheDir := t.TempDir()
	originalXDGHome := os.Getenv("XDG_CACHE_HOME")
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)
	defer t.Setenv("XDG_CACHE_HOME", originalXDGHome)

	t.Setenv("GITHUB_TOKEN", "fake-test-token")
	defer t.Setenv("GITHUB_TOKEN", "")

	ctx := context.Background()
	var client *github.Client
	var err error

	logMsgs := captureLogOutput(func() {
		client, err = ghclient.NewClient(ctx)
	})

	require.NoError(t, err)
	require.NotNil(t, client)

	assert.Contains(t, logMsgs, "Using GITHUB_TOKEN for authentication.")

	httpClient := client.Client()
	require.NotNil(t, httpClient)
	cachingTransport, ok := httpClient.Transport.(*ghclient.CachingTransport)
	require.True(t, ok, "Transport should be CachingTransport")
	_, ok = cachingTransport.Transport.(*oauth2.Transport)
	assert.True(t, ok, "CachingTransport should wrap oauth2.Transport when token is set")
}

func TestNewClient_WithoutToken(t *testing.T) {
	utils.CreateLogger(true)

	tempCacheDir := t.TempDir()
	originalXDGHome := os.Getenv("XDG_CACHE_HOME")
	t.Setenv("XDG_CACHE_HOME", tempCacheDir)
	defer t.Setenv("XDG_CACHE_HOME", originalXDGHome)

	originalToken := os.Getenv("GITHUB_TOKEN")
	t.Setenv("GITHUB_TOKEN", "")
	defer t.Setenv("GITHUB_TOKEN", originalToken)

	ctx := context.Background()
	var client *github.Client
	var err error

	logMsgs := captureLogOutput(func() {
		client, err = ghclient.NewClient(ctx)
	})

	require.NoError(t, err)
	require.NotNil(t, client)

	assert.Contains(t, logMsgs, "No GITHUB_TOKEN found")

	httpClient := client.Client()
	require.NotNil(t, httpClient)
	cachingTransport, ok := httpClient.Transport.(*ghclient.CachingTransport)
	require.True(t, ok, "Transport should be CachingTransport")
	_, ok = cachingTransport.Transport.(*oauth2.Transport)
	assert.False(t, ok, "CachingTransport should NOT wrap oauth2.Transport when token is not set")
}

func TestPrintRate(t *testing.T) {
	utils.CreateLogger(true)
	tests := []struct {
		name          string
		rate          *github.Rate
		expectedLogs  []string
		unexpectedLog string
	}{
		{
			name: "Authenticated",
			rate: &github.Rate{
				Limit:     5000,
				Remaining: 4000,
				Reset:     github.Timestamp{Time: time.Now().Add(10 * time.Minute)},
			},
			expectedLogs: []string{
				"Rate Limit:",
				"4000/5000 remaining",
				"Resets @",
				"Using authenticated rate limits.",
			},
		},
		{
			name: "Unauthenticated",
			rate: &github.Rate{
				Limit:     60,
				Remaining: 50,
				Reset:     github.Timestamp{Time: time.Now().Add(5 * time.Minute)},
			},
			expectedLogs: []string{
				"Rate Limit:",
				"50/60 remaining",
				"Resets @",
				"Using unauthenticated rate limits.",
			},
		},
		{
			name:          "Nil rate",
			rate:          nil,
			expectedLogs:  []string{"Rate limit info unavailable."},
			unexpectedLog: "Rate Limit:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *github.Response
			if tt.rate != nil {
				resp = &github.Response{Rate: *tt.rate}
			}

			logOutput := captureLogOutput(func() {
				ghclient.PrintRateLimit(resp)
			})

			for _, expected := range tt.expectedLogs {
				assert.Contains(t, logOutput, expected)
			}
			if tt.unexpectedLog != "" {
				assert.NotContains(t, logOutput, tt.unexpectedLog)
			}
		})
	}
}
