// SPDX-License-Identifier: MIT
package utils

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/shirou/gopsutil/v4/host"
)

var packageExts = map[string]string{
	"debian": "deb",
	"rhel":   "rpm",
	"suse":   "rpm",
	"alpine": "apk",
}

func DetectOS() (ext string) {
	_, family, _, err := host.PlatformInformation()
	if err != nil {
		Logger.Error(err)
		return ""
	}

	for _, v := range packageExts {
		if v == packageExts[family] {
			ext = v
		}
	}
	return ext
}

func GetExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ""
	}

	cleanExt := strings.TrimPrefix(ext, ".")

	// only return extensions that are actual system package types
	supportedExts := ListSupportedSystemPackages()
	if slices.Contains(supportedExts, cleanExt) {
		return cleanExt
	}

	// if not a supported system package extension, return empty
	return ""
}

func ListSupportedSystemPackages() []string {
	return []string{
		"deb", "rpm", "apk",
	}
}
