// SPDX-License-Identifier: MIT

package utils

import "testing"

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
