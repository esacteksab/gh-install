// SPDX-License-Identifier: MIT
package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func Test_saveAssetToFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "asset-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test data
	testData := []byte("This is test data for the download simulation")
	testSize := int64(len(testData))

	type args struct {
		rc          io.ReadCloser
		localPath   string
		displayName string
		assetSize   int64
	}

	tests := []struct {
		name    string
		args    args
		setup   func()
		wantErr bool
	}{
		{
			name: "successful download",
			args: args{
				rc:          io.NopCloser(bytes.NewReader(testData)),
				localPath:   filepath.Join(tempDir, "successful.txt"),
				displayName: "test-file.txt",
				assetSize:   testSize,
			},
			wantErr: false,
		},
		{
			name: "read error",
			args: args{
				rc:          io.NopCloser(&errorReader{err: errors.New("simulated read error")}),
				localPath:   filepath.Join(tempDir, "read-error.txt"),
				displayName: "error-file.txt",
				assetSize:   100, // Arbitrary size
			},
			wantErr: true,
		},
		{
			name: "invalid path",
			args: args{
				rc:          io.NopCloser(bytes.NewReader(testData)),
				localPath:   filepath.Join(tempDir, "non-existent-dir", "invalid.txt"),
				displayName: "invalid-path.txt",
				assetSize:   testSize,
			},
			wantErr: true,
		},
		{
			name: "zero-size file",
			args: args{
				rc:          io.NopCloser(bytes.NewReader([]byte{})),
				localPath:   filepath.Join(tempDir, "empty.txt"),
				displayName: "empty-file.txt",
				assetSize:   0,
			},
			wantErr: false,
		},
		{
			name: "close error",
			args: args{
				rc:          &errorCloser{Reader: bytes.NewReader(testData)},
				localPath:   filepath.Join(tempDir, "close-error.txt"),
				displayName: "close-error.txt",
				assetSize:   testSize,
			},
			wantErr: false, // The function continues even if close fails
		},
		{
			name: "path exists and is directory",
			args: args{
				rc:          io.NopCloser(bytes.NewReader(testData)),
				localPath:   tempDir, // Try to write to directory path
				displayName: "dir-path.txt",
				assetSize:   testSize,
			},
			setup: func() {
				// Ensure temp dir exists
				os.MkdirAll(tempDir, 0o755)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			err := saveAssetToFile(
				tt.args.rc,
				tt.args.localPath,
				tt.args.displayName,
				tt.args.assetSize,
			)

			// Check if error matches expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("saveAssetToFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// For successful cases, verify the file was created with correct content
			if !tt.wantErr && err == nil {
				// Check file exists
				fileInfo, err := os.Stat(tt.args.localPath)
				if err != nil {
					t.Errorf("Expected file %s to exist, but got error: %v", tt.args.localPath, err)
					return
				}

				// Check file size
				if tt.args.assetSize != fileInfo.Size() && tt.name != "close error" {
					t.Errorf("Expected file size %d, got %d", tt.args.assetSize, fileInfo.Size())
				}

				// For cases with testData, verify content
				if tt.args.assetSize > 0 {
					content, err := os.ReadFile(tt.args.localPath)
					if err != nil {
						t.Errorf("Failed to read file content: %v", err)
						return
					}

					if !bytes.Equal(content, testData) && tt.name != "close error" {
						t.Errorf("File content mismatch. Expected %s, got %s", testData, content)
					}
				}
			}
		})
	}
}

// Custom error types for testing

// errorReader is a reader that always returns an error
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

// errorCloser is a ReadCloser that returns an error on Close
type errorCloser struct {
	io.Reader
}

func (e *errorCloser) Close() error {
	return errors.New("simulated close error")
}
