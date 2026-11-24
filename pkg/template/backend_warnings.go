package template

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/abtreece/confd/pkg/backends"
	"github.com/abtreece/confd/pkg/log"
)

// backendKeyHints names the [backend] key to use for a spelling that
// confd.toml accepts but backends.Config does not decode.
var backendKeyHints = map[string]string{
	"client_ca_keys": "client_cakeys",
	"node":           "nodes",
}

// backendSectionWarnings returns warnings for the [backend] section of the
// resource file at path: keys the TOML decoder did not recognise, then
// settings the section's backend does not use (backends.IgnoredSettingWarnings).
// md is the metadata from decoding the file and cfg the decoded section, nil
// when the file has none. Each message names path.
//
// The decoder matches table and key names to struct tags case-insensitively,
// so [Backend] and Basic_Auth decode like [backend] and basic_auth, while the
// metadata keeps the spelling from the file. Names are therefore compared in
// lower case. Two spellings that differ only in case decode, or fail to
// decode, alike, so a lower-cased name is either defined or undecoded, never
// both.
func backendSectionWarnings(path string, md toml.MetaData, cfg *backends.Config) []string {
	if cfg == nil {
		return nil
	}

	defined := make(map[string]bool)
	for _, key := range md.Keys() {
		if len(key) == 2 && strings.EqualFold(key[0], "backend") {
			defined[strings.ToLower(key[1])] = true
		}
	}

	var warnings []string
	undecoded := make(map[string]bool)
	for _, key := range md.Undecoded() {
		if len(key) < 2 || !strings.EqualFold(key[0], "backend") {
			continue
		}
		name := strings.Join(key[1:], ".")
		undecoded[strings.ToLower(name)] = true
		warning := fmt.Sprintf("%s: unknown key %q in [backend] is ignored", path, name)
		if hint, ok := backendKeyHints[strings.ToLower(name)]; ok {
			warning += fmt.Sprintf("; use %q", hint)
		}
		warnings = append(warnings, warning)
	}

	if cfg.Backend == "" {
		return warnings
	}
	// An undecoded key has already been reported and never reached cfg, so
	// it must not also count as a supplied setting.
	supplied := func(key string) bool {
		return defined[key] && !undecoded[key]
	}
	for _, warning := range backends.IgnoredSettingWarnings(*cfg, supplied) {
		warnings = append(warnings, path+": "+warning)
	}
	return warnings
}

// warnBackendSection logs the backendSectionWarnings messages.
func warnBackendSection(path string, md toml.MetaData, cfg *backends.Config) {
	for _, warning := range backendSectionWarnings(path, md, cfg) {
		log.Warning("%s", warning)
	}
}
