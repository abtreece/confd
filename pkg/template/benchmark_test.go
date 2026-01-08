package template

import (
	"strings"
	"testing"
	"text/template"

	"github.com/abtreece/confd/pkg/memkv"
)

// Benchmark template functions

func BenchmarkSeq_Small(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Seq(1, 10)
	}
}

func BenchmarkSeq_Large(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Seq(1, 1000)
	}
}

func BenchmarkReverse_Strings(b *testing.B) {
	values := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Make a copy to avoid modifying the original
		v := make([]string, len(values))
		copy(v, values)
		Reverse(v)
	}
}

func BenchmarkReverse_KVPairs(b *testing.B) {
	values := []memkv.KVPair{
		{Key: "/a", Value: "1"},
		{Key: "/b", Value: "2"},
		{Key: "/c", Value: "3"},
		{Key: "/d", Value: "4"},
		{Key: "/e", Value: "5"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := make([]memkv.KVPair, len(values))
		copy(v, values)
		Reverse(v)
	}
}

func BenchmarkSortByLength(b *testing.B) {
	values := []string{"short", "a", "medium length", "longer string here", "b"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := make([]string, len(values))
		copy(v, values)
		SortByLength(v)
	}
}

func BenchmarkSortKVByLength(b *testing.B) {
	values := []memkv.KVPair{
		{Key: "/short", Value: "1"},
		{Key: "/a", Value: "2"},
		{Key: "/medium/length/key", Value: "3"},
		{Key: "/longer/path/here/now", Value: "4"},
		{Key: "/b", Value: "5"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v := make([]memkv.KVPair, len(values))
		copy(v, values)
		SortKVByLength(v)
	}
}

func BenchmarkBase64Encode(b *testing.B) {
	data := "Hello, World! This is a test string for base64 encoding."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Base64Encode(data)
	}
}

func BenchmarkBase64Decode(b *testing.B) {
	data := "SGVsbG8sIFdvcmxkISBUaGlzIGlzIGEgdGVzdCBzdHJpbmcgZm9yIGJhc2U2NCBlbmNvZGluZy4="
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Base64Decode(data)
	}
}

func BenchmarkUnmarshalJsonObject(b *testing.B) {
	data := `{"name": "test", "value": 123, "enabled": true, "tags": ["a", "b", "c"]}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		UnmarshalJsonObject(data)
	}
}

func BenchmarkUnmarshalJsonArray(b *testing.B) {
	data := `[{"name": "item1"}, {"name": "item2"}, {"name": "item3"}]`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		UnmarshalJsonArray(data)
	}
}

func BenchmarkCreateMap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CreateMap("key1", "value1", "key2", "value2", "key3", "value3")
	}
}

func BenchmarkGetenv_Exists(b *testing.B) {
	b.Setenv("BENCHMARK_TEST_VAR", "test_value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Getenv("BENCHMARK_TEST_VAR")
	}
}

func BenchmarkGetenv_NotExists_WithDefault(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Getenv("NONEXISTENT_VAR_12345", "default_value")
	}
}

// Benchmark template compilation and execution

func BenchmarkTemplateCompile_Small(b *testing.B) {
	tmplStr := `key: {{.Key}}
value: {{.Value}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		template.New("test").Parse(tmplStr)
	}
}

func BenchmarkTemplateCompile_Medium(b *testing.B) {
	tmplStr := `# Configuration file
{{range .Items}}
[{{.Name}}]
host = {{.Host}}
port = {{.Port}}
enabled = {{.Enabled}}
{{end}}

[global]
timeout = {{.Timeout}}
retries = {{.Retries}}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		template.New("test").Parse(tmplStr)
	}
}

func BenchmarkTemplateCompile_Large(b *testing.B) {
	// Build a larger template
	var sb strings.Builder
	sb.WriteString("# Large configuration\n")
	for i := 0; i < 50; i++ {
		sb.WriteString(`{{if .Enabled}}
[section` + string(rune('a'+i%26)) + `]
key = {{.Key}}
value = {{.Value}}
{{end}}
`)
	}
	tmplStr := sb.String()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		template.New("test").Parse(tmplStr)
	}
}

func BenchmarkTemplateExecute_Small(b *testing.B) {
	tmplStr := `key: {{.Key}}
value: {{.Value}}`
	tmpl, _ := template.New("test").Parse(tmplStr)
	data := map[string]string{"Key": "testkey", "Value": "testvalue"}
	var sb strings.Builder
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb.Reset()
		tmpl.Execute(&sb, data)
	}
}

func BenchmarkTemplateExecute_WithFuncs(b *testing.B) {
	funcs := newFuncMap()
	tmplStr := `key: {{base .Key}}
value: {{toUpper .Value}}
seq: {{range seq 1 5}}{{.}} {{end}}`
	tmpl, _ := template.New("test").Funcs(funcs).Parse(tmplStr)
	data := map[string]string{"Key": "/path/to/key", "Value": "testvalue"}
	var sb strings.Builder
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb.Reset()
		tmpl.Execute(&sb, data)
	}
}

func BenchmarkTemplateExecute_Range(b *testing.B) {
	tmplStr := `{{range .Items}}
name: {{.Name}}
value: {{.Value}}
{{end}}`
	tmpl, _ := template.New("test").Parse(tmplStr)
	items := make([]map[string]string, 20)
	for i := 0; i < 20; i++ {
		items[i] = map[string]string{"Name": "item", "Value": "value"}
	}
	data := map[string]interface{}{"Items": items}
	var sb strings.Builder
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb.Reset()
		tmpl.Execute(&sb, data)
	}
}

// Benchmark memkv store operations (used internally by templates)

func BenchmarkMemkvStore_Set(b *testing.B) {
	store := memkv.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Set("/test/key", "value")
	}
}

func BenchmarkMemkvStore_Get(b *testing.B) {
	store := memkv.New()
	store.Set("/test/key", "value")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get("/test/key")
	}
}

func BenchmarkMemkvStore_GetAll(b *testing.B) {
	store := memkv.New()
	for i := 0; i < 100; i++ {
		store.Set("/test/key"+string(rune('a'+i%26)), "value")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetAll("/test/*")
	}
}

func BenchmarkMemkvStore_SetMany(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store := memkv.New()
		for j := 0; j < 100; j++ {
			store.Set("/app/config/key"+string(rune('a'+j%26)), "value")
		}
	}
}

// Benchmark newFuncMap creation
func BenchmarkNewFuncMap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		newFuncMap()
	}
}
