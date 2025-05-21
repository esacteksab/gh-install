// SPDX-License-Identifier: MIT
package utils

import (
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
