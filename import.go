package gonfig

import (
	// Environment variable format support
	// Automatically registers env format decoder when imported
	_ "github.com/go-leo/gonfig/format/env"

	// JSON format support
	// Automatically registers json format decoder when imported
	_ "github.com/go-leo/gonfig/format/json"

	// TOML format support
	// Automatically registers toml format decoder when imported
	_ "github.com/go-leo/gonfig/format/toml"

	// YAML format support
	// Automatically registers yaml format decoder when imported
	_ "github.com/go-leo/gonfig/format/yaml"
)
