// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashFile(t *testing.T) {
	CreateLogger(true)
	fakeFile := filepath.Join(t.TempDir(), "fakeFile.txt") // Use t.TempDir()
	fakeContent := "not a checksum"
	anotherAnotherFakeFile := filepath.Join(
		t.TempDir(),
		"anotherAnotherFakeFile.txt",
	)
	moreFakeContent := "more fake content"

	err := os.WriteFile(fakeFile, []byte(fakeContent), 0o640)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	h, err := HashFile(fakeFile, "sha256")
	if err != nil {
		t.Fatalf("failed to hash file: %v", err)
	}

	err = os.WriteFile(anotherAnotherFakeFile, []byte(moreFakeContent), 0o740)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	// Note: Chmod to 000 might make the file unreadable by the test itself if it tries to re-open,
	// depending on the OS and user. HashFile will try to Open it.
	// This is a good test for open failure.
	err = os.Chmod(anotherAnotherFakeFile, 0o000)
	if err != nil {
		t.Logf(
			"Warning: could not chmod file to 000, test for non-available file might behave differently: %v",
			err,
		)
	}
	defer os.Chmod(
		anotherAnotherFakeFile,
		0o740,
	) // Attempt to restore permissions for cleanup by t.TempDir

	type args struct {
		assetPath string
		algorithm string
	}
	tests := []struct {
		name      string
		args      args
		want      string
		wantErr   bool
		preTestFn func(args) // Optional: for setup like chmod
	}{
		{
			name: "non-existent file",
			args: args{
				assetPath: filepath.Join(t.TempDir(), "reallyFakeFile.txt"),
				algorithm: "sha256",
			},
			want:    "",
			wantErr: true,
		},
		{
			name:    "file",
			args:    args{assetPath: fakeFile, algorithm: "sha256"},
			want:    h,
			wantErr: false,
		},
		{
			name:    "non-available file (permission denied)",
			args:    args{assetPath: anotherAnotherFakeFile, algorithm: "sha256"},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.preTestFn != nil {
				tt.preTestFn(tt.args)
			}
			got, err := HashFile(tt.args.assetPath, tt.args.algorithm)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("HashFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

// In utils_test.go

func TestVerifyChecksum(t *testing.T) {
	CreateLogger(true)

	assetContent := []byte("This is the content of our super important asset.")
	baseAssetPath := "my_asset_v1.0.0_linux_amd64.tar.gz" // This is the assetNameInChecksumFile

	tempDir := t.TempDir()
	assetPathOnDisk := filepath.Join(tempDir, baseAssetPath) // This is the assetPathOnDisk

	err := os.WriteFile(assetPathOnDisk, assetContent, 0o644)
	if err != nil {
		t.Fatalf("Setup: Failed to write asset file: %v", err)
	}

	expectedSha256, err := HashFile(assetPathOnDisk, "sha256")
	if err != nil {
		t.Fatalf("Setup: Failed to hash asset for sha256: %v", err)
	}
	expectedSha512, err := HashFile(assetPathOnDisk, "sha512")
	if err != nil {
		t.Fatalf("Setup: Failed to hash asset for sha512: %v", err)
	}

	t.Run("checksum file with algorithm in name", func(t *testing.T) {
		checksumFilePath := filepath.Join(
			tempDir,
			baseAssetPath+".sha256",
		) // This is the checksumFilePath
		checksumFileContent := fmt.Sprintf("%s  %s", expectedSha256, baseAssetPath)
		err := os.WriteFile(checksumFilePath, []byte(checksumFileContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write checksum file with algo: %v", err)
		}

		valid, algoUsed, err := VerifyChecksum(
			assetPathOnDisk,  // 1. Path to the asset to hash
			baseAssetPath,    // 2. Name of asset in checksum file
			checksumFilePath, // 3. Path to the checksum file itself
			"",               // 4. Default algo (empty means derive or error)
		)
		if err != nil {
			t.Errorf("VerifyChecksum() error = %v, wantErr nil", err)
		}
		if !valid {
			t.Errorf("VerifyChecksum() valid = %v, want true", valid)
		}
		if algoUsed != "sha256" {
			t.Errorf("VerifyChecksum() algoUsed = %s, want 'sha256'", algoUsed)
		}
	})

	t.Run("generic checksum file with SHA256 default", func(t *testing.T) {
		checksumFilePath := filepath.Join(
			tempDir,
			"myproject_1.0.0_checksums.txt",
		) // This is the checksumFilePath
		genericChecksumFileContent := fmt.Sprintf(
			"%s  %s\n# Some other file\n%s  some_other_file.zip",
			expectedSha256, baseAssetPath, "fakechecksumforsomeotherfile")
		err := os.WriteFile(checksumFilePath, []byte(genericChecksumFileContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write generic checksum file: %v", err)
		}

		valid, algoUsed, err := VerifyChecksum(
			assetPathOnDisk,                     // 1.
			baseAssetPath,                       // 2.
			checksumFilePath,                    // 3.
			DefaultAlgorithmForGenericChecksums, // 4.
		)
		if err != nil {
			t.Errorf("VerifyChecksum() error = %v, wantErr nil", err)
		}
		if !valid {
			t.Errorf("VerifyChecksum() valid = %v, want true", valid)
		}
		if algoUsed != DefaultAlgorithmForGenericChecksums {
			t.Errorf(
				"VerifyChecksum() algoUsed = %s, want '%s'",
				algoUsed,
				DefaultAlgorithmForGenericChecksums,
			)
		}
	})

	t.Run("generic checksum file with SHA512 hint", func(t *testing.T) {
		checksumFilePath := filepath.Join(
			tempDir,
			"myproject_1.0.0_special_checksums.txt",
		) // This is checksumFilePath
		genericSha512ChecksumFileContent := fmt.Sprintf("%s  %s", expectedSha512, baseAssetPath)
		err := os.WriteFile(checksumFilePath, []byte(genericSha512ChecksumFileContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to write generic SHA512 checksum file: %v", err)
		}

		valid, algoUsed, err := VerifyChecksum(
			assetPathOnDisk,  // 1.
			baseAssetPath,    // 2.
			checksumFilePath, // 3.
			"sha512",         // 4. Hint/default algo
		)
		if err != nil {
			t.Errorf("VerifyChecksum() error = %v, wantErr nil", err)
		}
		if !valid {
			t.Errorf("VerifyChecksum() valid = %v, want true", valid)
		}
		if algoUsed != "sha512" {
			t.Errorf("VerifyChecksum() algoUsed = %s, want 'sha512'", algoUsed)
		}
	})

	t.Run("deliberate mismatch with different asset", func(t *testing.T) {
		mismatchAssetPathOnDisk := filepath.Join(
			tempDir,
			"mismatch_asset.dat",
		) // This is assetPathOnDisk
		err := os.WriteFile(mismatchAssetPathOnDisk, []byte("different content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to write mismatch asset: %v", err)
		}

		// Checksum file contains checksum for original 'baseAssetPath'
		checksumFilePath := filepath.Join(tempDir, baseAssetPath+".sha256")
		checksumContentForOriginal := fmt.Sprintf("%s  %s", expectedSha256, baseAssetPath)
		err = os.WriteFile(checksumFilePath, []byte(checksumContentForOriginal), 0o644)
		if err != nil {
			t.Fatalf("Failed to write checksum file for original asset: %v", err)
		}

		// We are trying to verify mismatchAssetPathOnDisk
		// but the checksum file (checksumFilePath) lists baseAssetPath
		// So, for ParseChecksumFile to fail to find 'mismatch_asset.dat',
		// assetNameInChecksumFile should be filepath.Base(mismatchAssetPathOnDisk)
		valid, algoUsed, err := VerifyChecksum(
			mismatchAssetPathOnDisk, // 1. Asset to hash
			filepath.Base(
				mismatchAssetPathOnDisk,
			), // 2. Name to look for in checksum file ("mismatch_asset.dat")
			checksumFilePath, // 3. Path to checksum file (which lists 'baseAssetPath', not 'mismatch_asset.dat')
			"",               // 4. Default algo
		)

		if err == nil {
			t.Errorf("VerifyChecksum() error = nil, want an error")
		} else {
			// Expecting "checksum for target 'mismatch_asset.dat' not found..."
			if !strings.Contains(err.Error(), "not found in checksum file") {
				t.Errorf("VerifyChecksum() error = %v, want error containing 'not found in checksum file'", err)
			}
			if !strings.Contains(err.Error(), "mismatch_asset.dat") {
				t.Errorf("VerifyChecksum() error = %v, want error mentioning 'mismatch_asset.dat'", err)
			}
		}
		if valid {
			t.Errorf("VerifyChecksum() valid = %v, want false", valid)
		}
		// algoUsed should be "sha256" as derived from checksumFilePath, even if parsing failed later
		if algoUsed != "sha256" {
			t.Errorf("VerifyChecksum() algoUsed = %s, want 'sha256'", algoUsed)
		}
	})

	t.Run("target checksum not in file", func(t *testing.T) {
		checksumFilePath := filepath.Join(tempDir, "non_matching_checksums.txt")
		content := "unrelatedchecksum  unrelatedfile.zip\n"
		err := os.WriteFile(checksumFilePath, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write non-matching checksum file: %v", err)
		}

		valid, algoUsed, err := VerifyChecksum(
			assetPathOnDisk,                     // 1. Asset to hash
			baseAssetPath,                       // 2. Name to look for (won't be in nonMatchingChecksumFile)
			checksumFilePath,                    // 3. Path to the checksum file
			DefaultAlgorithmForGenericChecksums, // 4.
		)

		if err == nil {
			t.Errorf("VerifyChecksum() error = nil, want an error for 'not found'")
		} else {
			if !strings.Contains(err.Error(), "not found in checksum file") {
				t.Errorf("VerifyChecksum() error = %v, want error containing 'not found in checksum file'", err)
			}
		}
		if valid {
			t.Errorf("VerifyChecksum() valid = %v, want false", valid)
		}
		if algoUsed != DefaultAlgorithmForGenericChecksums {
			t.Errorf(
				"VerifyChecksum() algoUsed = %s, want '%s'",
				algoUsed,
				DefaultAlgorithmForGenericChecksums,
			)
		}
	})

	t.Run("unsupported algorithm from filename", func(t *testing.T) {
		checksumFilePath := filepath.Join(tempDir, baseAssetPath+".unsupported")
		content := fmt.Sprintf("checksum  %s\n", baseAssetPath)
		err := os.WriteFile(checksumFilePath, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write unsupported algo checksum file: %v", err)
		}

		savedAlgorithmExts := algorithmExts
		tempModifiedExts := make(map[string]bool)
		maps.Copy(tempModifiedExts, savedAlgorithmExts)
		tempModifiedExts[".unsupported"] = true
		algorithmExts = tempModifiedExts
		defer func() { algorithmExts = savedAlgorithmExts }()

		valid, algoUsed, err := VerifyChecksum(
			assetPathOnDisk,  // 1.
			baseAssetPath,    // 2.
			checksumFilePath, // 3.
			"",               // 4. (empty because algo should be derived from ext)
		)

		if err == nil {
			t.Errorf("VerifyChecksum() error = nil, want an error for 'unsupported algorithm'")
		} else {
			if !strings.Contains(err.Error(), "is not supported") {
				t.Errorf("VerifyChecksum() error = %v, want error containing 'is not supported'", err)
			}
			if !strings.Contains(err.Error(), "unsupported") {
				t.Errorf("VerifyChecksum() error = %v, want error mentioning 'unsupported'", err)
			}
		}
		if valid {
			t.Errorf("VerifyChecksum() valid = %v, want false", valid)
		}
		if algoUsed != "unsupported" {
			t.Errorf("VerifyChecksum() algoUsed = %s, want 'unsupported'", algoUsed)
		}
	})
}

func TestHashFileAllTheAlgo(t *testing.T) {
	CreateLogger(false)
	// --- Setup a dummy file for testing ---
	dummyFilePath := "testcontent.dat"
	dummyFileContent := []byte("This is some test data for hashing.\nGoReleaser is awesome!")
	if err := os.WriteFile(dummyFilePath, dummyFileContent, 0o644); err != nil {
		t.Fatalf("Failed to create dummy file '%s': %v", dummyFilePath, err)
	}
	defer os.Remove(dummyFilePath)

	// --- Test HashFile with all supported algorithms ---
	fmt.Println("--- Hashing dummy file with various algorithms ---")
	for _, algo := range ListSupportedAlgorithms() {
		checksum, err := HashFile(dummyFilePath, algo)
		if err != nil {
			t.Fatalf("ERROR hashing '%s' with %s: %v", dummyFilePath, algo, err)
		} else {
			fmt.Printf("SUCCESS: Algo: %-10s File: %s Checksum: %s", algo, dummyFilePath, checksum)
		}
	}

	// --- Test IsChecksumFile and GetAlgorithmFromFilename ---
	fmt.Println("\n--- Testing file type identification ---")
	testFiles := []struct {
		path         string
		isChecksum   bool   // Expected from IsChecksumFile
		expectedAlgo string // Expected from GetAlgorithmFromFilename (if isChecksum is true and based on ext)
	}{
		{"archive.zip.sha256", true, "sha256"},
		{"my-app.exe.md5", true, "md5"},
		{"document.pdf", false, ""},
		{"checksums.txt", true, ""}, // IsChecksumFile=true (regex), GetAlgorithmFromFilename=false
		{"SHA512SUMS", true, ""},    // IsChecksumFile=true (regex), GetAlgorithmFromFilename=false
		{"data.bin.blake2b", true, "blake2b"},
		{"backup.tar.gz.sha3-512", true, "sha3-512"},
		{"README.md", false, ""},
		{"config.json.crc32", true, "crc32"},
		{"file.with.dots.sha1", true, "sha1"},
		{"image.jpeg", false, ""},
	}

	for _, tf := range testFiles {
		is := IsChecksumFile(tf.path)
		fmt.Printf("File: '%-25s' -> IsChecksumFile: %-5t", tf.path, is)
		if is != tf.isChecksum {
			t.Fatalf(" (Expected: %t) <<< MISMATCH", tf.isChecksum)
		}

		derivedAlgo, algoFound := GetAlgorithmFromFilename(tf.path)
		if algoFound {
			fmt.Printf(" -> Derived Algo: '%s'", derivedAlgo)
			if derivedAlgo != tf.expectedAlgo {
				t.Fatalf(" (Expected: '%s') <<< MISMATCH", tf.expectedAlgo)
			}
		} else if tf.expectedAlgo != "" {
			// This means we expected an algo from extension, but didn't get one
			t.Fatalf(" -> No algo derived (Expected: '%s') <<< MISMATCH", tf.expectedAlgo)
		} else if is && tf.expectedAlgo == "" {
			// IsChecksumFile true (likely from regex), and we correctly didn't derive an algo from ext
			fmt.Printf(" -> No algo derived from ext (as expected for manifest file)")
		}
	}

	// --- Test error cases for HashFile ---
	fmt.Println("\n--- Testing HashFile error cases ---")
	_, err := HashFile(dummyFilePath, "unknown-algo")
	if err != nil {
		fmt.Printf("SUCCESS: Correctly failed for unknown algorithm: %v", err)
	} else {
		t.Fatal("ERROR: HashFile should have failed for 'unknown-algo'")
	}

	_, err = HashFile("nonexistent-file.txt", "sha256")
	if err != nil {
		fmt.Printf("SUCCESS: Correctly failed for non-existent file: %v", err)
	} else {
		t.Fatal("ERROR: HashFile should have failed for 'nonexistent-file.txt'")
	}
}
