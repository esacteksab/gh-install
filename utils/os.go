// SPDX-License-Identifier: MIT
package utils

import (
	"path/filepath"
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
	return strings.TrimPrefix(ext, ".")
}

func ListSupportedSystemPackages() []string {
	return []string{
		"deb", "rpm", "apk",
	}
}
