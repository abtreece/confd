package env

import (
	"context"
	"os"
	"testing"
)

func BenchmarkGetValues_SingleKey(b *testing.B) {
	client, _ := NewEnvClient()
	os.Setenv("BENCH_TEST_KEY", "test_value")
	defer os.Unsetenv("BENCH_TEST_KEY")

	keys := []string{"/bench/test/key"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetValues(context.Background(), keys)
	}
}

func BenchmarkGetValues_MultipleKeys(b *testing.B) {
	client, _ := NewEnvClient()
	for i := 0; i < 10; i++ {
		os.Setenv("BENCH_KEY_"+string(rune('A'+i)), "value")
	}
	defer func() {
		for i := 0; i < 10; i++ {
			os.Unsetenv("BENCH_KEY_" + string(rune('A'+i)))
		}
	}()

	keys := []string{
		"/bench/key/a", "/bench/key/b", "/bench/key/c",
		"/bench/key/d", "/bench/key/e", "/bench/key/f",
		"/bench/key/g", "/bench/key/h", "/bench/key/i", "/bench/key/j",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetValues(context.Background(), keys)
	}
}

func BenchmarkTransform(b *testing.B) {
	for i := 0; i < b.N; i++ {
		transform("/app/database/host")
	}
}

func BenchmarkClean(b *testing.B) {
	for i := 0; i < b.N; i++ {
		clean("APP_DATABASE_HOST")
	}
}

func BenchmarkNewEnvClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewEnvClient()
	}
}
