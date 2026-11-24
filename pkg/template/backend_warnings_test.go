package template

import (
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestBackendSectionWarnings(t *testing.T) {
	const path = "/etc/confd/conf.d/app.toml"
	const resourceHeader = "[template]\nsrc = \"app.tmpl\"\ndest = \"/tmp/app.conf\"\n"

	tests := []struct {
		name    string
		backend string // the [backend] section; empty means none
		want    []string
	}{
		{
			name: "no [backend] section",
		},
		{
			name:    "clean section",
			backend: "backend = \"consul\"\nnodes = [\"127.0.0.1:8500\"]\nbasic_auth = true\nusername = \"u\"\nclient_cakeys = \"/ca\"",
		},
		{
			name:    "confd.toml alias client_ca_keys",
			backend: "backend = \"consul\"\nclient_ca_keys = \"/ca\"",
			want:    []string{path + `: unknown key "client_ca_keys" in [backend] is ignored; use "client_cakeys"`},
		},
		{
			name:    "confd.toml alias node",
			backend: "backend = \"consul\"\nnode = [\"127.0.0.1:8500\"]",
			want:    []string{path + `: unknown key "node" in [backend] is ignored; use "nodes"`},
		},
		{
			name:    "typo without a hint",
			backend: "backend = \"consul\"\nbasic-auth = true",
			want:    []string{path + `: unknown key "basic-auth" in [backend] is ignored`},
		},
		{
			name:    "setting the backend does not use",
			backend: "backend = \"redis\"\nbasic_auth = true",
			want:    []string{path + ": --basic-auth has no effect with the redis backend"},
		},
		{
			name:    "consul username without basic_auth",
			backend: "backend = \"consul\"\nusername = \"u\"",
			want:    []string{path + ": --username has no effect with the consul backend unless --basic-auth is set"},
		},
		{
			name:    "undecoded alias warns once, not also as unused",
			backend: "backend = \"redis\"\nclient_ca_keys = \"/ca\"",
			want:    []string{path + `: unknown key "client_ca_keys" in [backend] is ignored; use "client_cakeys"`},
		},
		{
			name:    "unknown keys before unused settings",
			backend: "backend = \"redis\"\nauth_token = \"t\"\nbasic-auth = true",
			want: []string{
				path + `: unknown key "basic-auth" in [backend] is ignored`,
				path + ": --auth-token has no effect with the redis backend",
			},
		},
		{
			name:    "empty backend reports only unknown keys",
			backend: "basic_auth = true\nclient_ca_keys = \"/ca\"",
			want:    []string{path + `: unknown key "client_ca_keys" in [backend] is ignored; use "client_cakeys"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := resourceHeader
			if tt.backend != "" {
				doc += "\n[backend]\n" + tt.backend + "\n"
			}
			var tc TemplateResourceConfig
			md, err := toml.Decode(doc, &tc)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			got := backendSectionWarnings(path, md, tc.BackendConfig)
			if !slices.Equal(got, tt.want) {
				t.Errorf("warnings:\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

// TestBackendSectionWarningsCaseInsensitive covers spellings the decoder
// accepts case-insensitively while the metadata keeps them as written.
func TestBackendSectionWarningsCaseInsensitive(t *testing.T) {
	const path = "/etc/confd/conf.d/app.toml"

	tests := []struct {
		name string
		doc  string
		want []string
	}{
		{
			name: "[Backend] table with an unknown key",
			doc:  "[Backend]\nbackend = \"consul\"\nclient_ca_keys = \"/ca\"\n",
			want: []string{path + `: unknown key "client_ca_keys" in [backend] is ignored; use "client_cakeys"`},
		},
		{
			name: "[BACKEND] table with an unused setting",
			doc:  "[BACKEND]\nbackend = \"redis\"\nbasic_auth = true\n",
			want: []string{path + ": --basic-auth has no effect with the redis backend"},
		},
		{
			name: "mixed-case setting the backend does not use",
			doc:  "[backend]\nbackend = \"file\"\nBasic_Auth = true\n",
			want: []string{path + ": --basic-auth has no effect with the file backend"},
		},
		{
			name: "mixed-case alias keeps its spelling and gets the hint",
			doc:  "[backend]\nbackend = \"redis\"\nClient_Ca_Keys = \"/ca\"\n",
			want: []string{path + `: unknown key "Client_Ca_Keys" in [backend] is ignored; use "client_cakeys"`},
		},
		{
			name: "mixed-case setting the backend uses",
			doc:  "[backend]\nbackend = \"consul\"\nBASIC_AUTH = true\nUsername = \"u\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tc TemplateResourceConfig
			md, err := toml.Decode(tt.doc, &tc)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if tc.BackendConfig == nil {
				t.Fatal("the [backend] table was not decoded")
			}
			got := backendSectionWarnings(path, md, tc.BackendConfig)
			if !slices.Equal(got, tt.want) {
				t.Errorf("warnings:\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

