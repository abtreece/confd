package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkGetValues_SmallYAML(b *testing.B) {
	tmpDir := b.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	content := `
database:
  host: localhost
  port: 5432
  name: mydb
`
	os.WriteFile(yamlFile, []byte(content), 0644)

	client, _ := NewFileClient([]string{yamlFile}, "")
	keys := []string{"/database"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetValues(context.Background(), keys)
	}
}

func BenchmarkGetValues_MediumYAML(b *testing.B) {
	tmpDir := b.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")
	content := `
database:
  host: localhost
  port: 5432
  name: mydb
  user: admin
  password: secret
  pool_size: 10
  timeout: 30
cache:
  enabled: true
  ttl: 3600
  max_size: 1000
logging:
  level: info
  format: json
  output: stdout
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30
  write_timeout: 30
`
	os.WriteFile(yamlFile, []byte(content), 0644)

	client, _ := NewFileClient([]string{yamlFile}, "")
	keys := []string{"/"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetValues(context.Background(), keys)
	}
}

func BenchmarkGetValues_LargeYAML(b *testing.B) {
	tmpDir := b.TempDir()
	yamlFile := filepath.Join(tmpDir, "config.yaml")

	// Generate a larger YAML file
	content := "services:\n"
	for i := 0; i < 50; i++ {
		content += "  service" + string(rune('a'+i%26)) + ":\n"
		content += "    host: localhost\n"
		content += "    port: " + string(rune('0'+i%10)) + "000\n"
		content += "    enabled: true\n"
		content += "    timeout: 30\n"
	}
	os.WriteFile(yamlFile, []byte(content), 0644)

	client, _ := NewFileClient([]string{yamlFile}, "")
	keys := []string{"/services"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetValues(context.Background(), keys)
	}
}

func BenchmarkGetValues_JSON(b *testing.B) {
	tmpDir := b.TempDir()
	jsonFile := filepath.Join(tmpDir, "config.json")
	content := `{
  "database": {
    "host": "localhost",
    "port": 5432,
    "name": "mydb"
  },
  "cache": {
    "enabled": true,
    "ttl": 3600
  }
}`
	os.WriteFile(jsonFile, []byte(content), 0644)

	client, _ := NewFileClient([]string{jsonFile}, "")
	keys := []string{"/"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetValues(context.Background(), keys)
	}
}

func BenchmarkGetValues_MultipleFiles(b *testing.B) {
	tmpDir := b.TempDir()
	files := make([]string, 5)
	for i := 0; i < 5; i++ {
		files[i] = filepath.Join(tmpDir, "config"+string(rune('a'+i))+".yaml")
		content := "key" + string(rune('a'+i)) + ": value" + string(rune('a'+i)) + "\n"
		os.WriteFile(files[i], []byte(content), 0644)
	}

	client, _ := NewFileClient(files, "")
	keys := []string{"/"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetValues(context.Background(), keys)
	}
}

func BenchmarkNewFileClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewFileClient([]string{"/tmp/config.yaml"}, "*.yaml")
	}
}

func BenchmarkNodeWalk_Flat(b *testing.B) {
	node := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vars := make(map[string]string)
		nodeWalk(node, "/", vars)
	}
}

func BenchmarkNodeWalk_Nested(b *testing.B) {
	node := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"key": "value",
				},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vars := make(map[string]string)
		nodeWalk(node, "/", vars)
	}
}

func BenchmarkNodeWalk_Array(b *testing.B) {
	node := map[string]interface{}{
		"items": []interface{}{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vars := make(map[string]string)
		nodeWalk(node, "/", vars)
	}
}
