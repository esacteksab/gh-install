// SPDX-License-Identifier: MIT

package config

import (
	kt "github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type BinaryConfig struct {
	Key     string `koanf:"key"`
	Name    string `koanf:"name"`
	Version string `koanf:"version"`
}

type Config struct {
	Binaries map[string]BinaryConfig `koanf:"binaries"`
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
