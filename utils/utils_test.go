// SPDX-License-Identifier: MIT

package utils

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

func Test_IsChecksumFile(t *testing.T) {
	type args struct {
		file string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// Existing test cases
		{
			name: "is checksum",
			args: args{"checksum.txt"},
			want: true,
		},
		{
			name: "is Checksum",
			args: args{"Checksum.txt"},
			want: true,
		},
		{
			name: "is not checksum",
			args: args{"not.txt"},
			want: false,
		},
		{
			name: "is checksum_v0.1.1",
			args: args{"binary_checksum_v0.1.1.txt"},
			want: true,
		},
		{
			name: "is v0.1.1_checksum",
			args: args{"binary_v0.1.1_checksum.txt"},
			want: true,
		},
		{
			name: "is checksum",
			args: args{"checksum"},
			want: true,
		},
		{
			name: "is 256",
			args: args{"binary.sha256"},
			want: true,
		},

		// Additional test cases for different algorithms
		{
			name: "is sha512",
			args: args{"binary.sha512"},
			want: true,
		},
		{
			name: "is sha1",
			args: args{"binary.sha1"},
			want: true,
		},
		{
			name: "is crc32",
			args: args{"binary.crc32"},
			want: true,
		},
		{
			name: "is md5",
			args: args{"binary.md5"},
			want: true,
		},
		{
			name: "is sha224",
			args: args{"binary.sha224"},
			want: true,
		},
		{
			name: "is sha384",
			args: args{"binary.sha384"},
			want: true,
		},
		{
			name: "is sha3-256",
			args: args{"binary.sha3-256"},
			want: true,
		},
		{
			name: "is sha3-512",
			args: args{"binary.sha3-512"},
			want: true,
		},
		{
			name: "is sha3-224",
			args: args{"binary.sha3-224"},
			want: true,
		},
		{
			name: "is sha3-384",
			args: args{"binary.sha3-384"},
			want: true,
		},
		{
			name: "is blake2s",
			args: args{"binary.blake2s"},
			want: true,
		},
		{
			name: "is blake2b",
			args: args{"binary.blake2b"},
			want: true,
		},

		// Additional tests for uppercase variants
		{
			name: "is SHA256 uppercase",
			args: args{"binary.SHA256"},
			want: true,
		},
		{
			name: "is MD5 uppercase",
			args: args{"binary.MD5"},
			want: true,
		},

		// Additional tests for mixed case variants
		{
			name: "is Sha256 mixed case",
			args: args{"binary.Sha256"},
			want: true,
		},

		// Tests for files with uppercase names
		{
			name: "is uppercase filename with algorithm",
			args: args{"BINARY.sha256"},
			want: true,
		},

		// Tests for files with version numbers
		{
			name: "is version with algorithm",
			args: args{"binary-v1.2.3.sha256"},
			want: true,
		},

		// Test for files with underscores
		{
			name: "is underscore with algorithm",
			args: args{"binary_linux_amd64.sha256"},
			want: true,
		},

		// Test for non-checksum files
		{
			name: "is not checksum file",
			args: args{"binary.exe"},
			want: false,
		},
		{
			name: "is not checksum file with text extension",
			args: args{"readme.txt"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsChecksumFile(tt.args.file); got != tt.want {
				t.Errorf("IsChecksumFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchFile(t *testing.T) {
	CreateLogger(true)
	GetOSArch()
	type args struct {
		file string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "windows",
			args: args{"binary_v0.0.1_windows-amd64"},
			want: false,
		},
		{
			name: "darwin",
			args: args{"binary_v0.0.1_darwin-arm64"},
			want: false,
		},
		{
			name: "linux",
			args: args{"binary_v0.0.1_linux-amd64"},
			want: true,
		},
		{
			name: "Linux",
			args: args{"binary_v0.0.1_Linux-amd64"},
			want: true,
		},
		{
			name: "LINUX",
			args: args{"BINARY_0.0.1_LINUX-AMD64"},
			want: true,
		},
		{
			name: "LINUX_x86_64",
			args: args{"BINARY_0.0.1_LINUX-x86_64"},
			want: true,
		},
		{
			name: "LINUX_i386",
			args: args{"BINARY_0.0.1_LINUX-i386"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchFile(tt.args.file); got != tt.want {
				t.Errorf("MatchFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchFileNoGetOSArch(t *testing.T) {
	CreateLogger(true)

	resetOsArchRegexesForTesting()

	type args struct {
		file string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "windows",
			args: args{"binary_v0.0.1_linux-amd64"},
			want: false, // this returns false because GetOSArch() is nil
			// see L183 above where it *is* called
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchFile(tt.args.file); got != tt.want {
				t.Errorf("MatchFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseArgs(t *testing.T) {
	CreateLogger(true)
	type args struct {
		argString string
	}
	tests := []struct {
		name    string
		args    args
		want    ParsedArgs
		wantErr bool
	}{
		{
			name:    "owner/repo@tag",
			args:    args{argString: "owner/repo@tag"},
			want:    ParsedArgs{Owner: "owner", Repo: "repo", Version: "tag"},
			wantErr: false,
		},
		{
			name:    "owner/repo@latest",
			args:    args{argString: "owner/repo@latest"},
			want:    ParsedArgs{Owner: "owner", Repo: "repo", Version: "latest"},
			wantErr: false,
		},
		{
			name:    "owner/repo@",
			args:    args{argString: "owner/repo@"},
			want:    ParsedArgs{},
			wantErr: true,
		},
		{
			name:    "owner/repo",
			args:    args{argString: "owner/repo"},
			want:    ParsedArgs{Owner: "owner", Repo: "repo", Version: "latest"},
			wantErr: false,
		},
		{
			name:    "owner/",
			args:    args{argString: "owner/"},
			want:    ParsedArgs{},
			wantErr: true,
		},
		{
			name:    "/repo",
			args:    args{argString: "/repo"},
			want:    ParsedArgs{},
			wantErr: true,
		},
		{
			name:    "@latest",
			args:    args{argString: "@latest"},
			want:    ParsedArgs{},
			wantErr: true,
		},
		{
			name:    "empty",
			args:    args{argString: ""},
			want:    ParsedArgs{},
			wantErr: true,
		},
		{
			name:    "owner/repo/foo@latest",
			args:    args{argString: "owner/repo/foo@latest"},
			want:    ParsedArgs{},
			wantErr: true,
		},
		{
			name:    "owner/repo@foo@latest",
			args:    args{argString: "owner/repo@foo@latest"},
			want:    ParsedArgs{},
			wantErr: true,
		},
		{
			name:    "owner/repo/foo/latest",
			args:    args{argString: "owner/repo/foo/latest"},
			want:    ParsedArgs{},
			wantErr: true,
		},
		{
			name:    "owner\repo\\latest",
			args:    args{argString: "owner\repo\foo\\latest"},
			want:    ParsedArgs{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseArgs(tt.args.argString)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOSArch(t *testing.T) {
	CreateLogger(false)
	tests := []struct {
		name string
	}{
		{
			name: "linux/amd64",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			GetOSArch()
		})
	}
}

func TestParseChecksumFile(t *testing.T) {
	CreateLogger(true)

	checksumFile := "checksum.txt"
	notACheckSumFile := "fakeFile.txt"
	fakeCheckSum := "not a checksum"
	emptyLineFile := "emptyLine.txt"
	emptyLine := "\n# an empty line\nNot an empty line"
	malformedLineFile := "malformed.txt"
	malformedLine := "a19aed32eaecf9f67274abd4e96fa97955ae82d04b37ff749f2e7b39815ef15c"

	err := os.WriteFile(notACheckSumFile, []byte(fakeCheckSum), 0o640)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(notACheckSumFile)

	notACheckSumFileHash, err := HashFile(notACheckSumFile, "sha256")
	if err != nil {
		t.Fatalf("failed to hash file: %v", err)
	}
	checksum := fmt.Sprintf("%s %s", notACheckSumFileHash, notACheckSumFile)

	err = os.WriteFile(checksumFile, []byte(checksum), 0o644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(checksumFile)

	err = os.WriteFile(emptyLineFile, []byte(checksum+emptyLine), 0o640)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(emptyLineFile)

	err = os.WriteFile(malformedLineFile, []byte(malformedLine), 0o640)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(malformedLineFile)

	type args struct {
		checksumFilePath string
		targetFilename   string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "not a checksum file",
			args:    args{checksumFilePath: "fakeFile.txt", targetFilename: "nonexistentFile"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "a checksum file",
			args:    args{checksumFilePath: "checksum.txt", targetFilename: "fakeFile.txt"},
			want:    notACheckSumFileHash,
			wantErr: false,
		},
		{
			name:    "an empty line",
			args:    args{checksumFilePath: "emptyLine.txt", targetFilename: "fakeFile.txt"},
			want:    notACheckSumFileHash,
			wantErr: false,
		},
		{
			name: "a malformed line",
			args: args{
				checksumFilePath: "malformed.txt",
				targetFilename:   "fakeFile.txt",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "a non-existent checksum file",
			args: args{
				checksumFilePath: "doesntExist.txt",
				targetFilename:   "fakeFile.txt",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseChecksumFile(tt.args.checksumFilePath, tt.args.targetFilename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseChecksumFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseChecksumFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseBinaryName(t *testing.T) {
	type args struct {
		file string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "underscores only",
			args: args{"binary_v0.0.1_windows-amd64"},
			want: "binary",
		},
		{
			name: "hyphens only",
			args: args{"binary-v0.0.1-windows-amd64"},
			want: "binary",
		},
		{
			name: "underscores only",
			args: args{"binary_0.0.1_windows-amd64"},
			want: "binary",
		},
		{
			name: "hyphens only",
			args: args{"binary-0.0.1-windows-amd64"},
			want: "binary",
		},
		{
			name: "hyphenated binary with underscores",
			args: args{"binary-foo_v0.0.1_windows_amd64"},
			want: "binary-foo",
		},
		{
			name: "hyphenated binary with hyphens",
			args: args{"binary-foo-v0.0.1-windows-amd64"},
			want: "binary-foo",
		},
		{
			name: "hyphenated binary with hyphens no v",
			args: args{"binary-foo-0.0.1-windows-amd64"},
			want: "binary-foo",
		},
		{
			name: "hyphenated binary with hyphens with v",
			args: args{"binary-foo-v0.0.1-windows-amd64"},
			want: "binary-foo",
		},
		{
			name: "no OS or arch",
			args: args{"binary"},
			want: "binary",
		},
		{
			name: "no version",
			args: args{"binary_windows_amd64"},
			want: "binary",
		},
		{
			name: "no version with hyphens",
			args: args{"binary-windows-amd64"},
			want: "binary",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseBinaryName(tt.args.file); got != tt.want {
				t.Errorf("ParseBinary() = %v, want %v", got, tt.want)
			}
		})
	}
}
