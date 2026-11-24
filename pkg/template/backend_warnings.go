package template

import (
	"fmt"
	"slices"
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
// The decoder matches table and key names to struct tags with
// strings.EqualFold, so [Backend], Basic_Auth and even "baſic_auth" (U+017F)
// decode like [backend] and basic_auth, while the metadata keeps the spelling
// from the file. Names are therefore compared with strings.EqualFold too;
// lower-casing is not equivalent, since Unicode case folding also maps
// characters such as ſ to s.
func backendSectionWarnings(path string, md toml.MetaData, cfg *backends.Config) []string {
	if cfg == nil {
		return nil
	}

	var defined []string
	for _, key := range md.Keys() {
		if len(key) == 2 && strings.EqualFold(key[0], "backend") {
			defined = append(defined, key[1])
		}
	}

	var warnings []string
	var undecoded []string
	for _, key := range md.Undecoded() {
		if len(key) < 2 || !strings.EqualFold(key[0], "backend") {
			continue
		}
		name := strings.Join(key[1:], ".")
		undecoded = append(undecoded, name)
		warning := fmt.Sprintf("%s: unknown key %q in [backend] is ignored", path, name)
		for alias, hint := range backendKeyHints {
			if strings.EqualFold(name, alias) {
				warning += fmt.Sprintf("; use %q", hint)
				break
			}
		}
		warnings = append(warnings, warning)
	}

	if cfg.Backend == "" {
		return warnings
	}
	// An undecoded key has already been reported and never reached cfg, so
	// it must not also count as a supplied setting.
	supplied := func(key string) bool {
		matches := func(name string) bool { return strings.EqualFold(name, key) }
		return slices.ContainsFunc(defined, matches) && !slices.ContainsFunc(undecoded, matches)
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
