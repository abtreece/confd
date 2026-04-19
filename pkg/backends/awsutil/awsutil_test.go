// Use white-box package to call LoadAWSConfig directly without import qualifier.
package awsutil

import (
	"context"
	"os"
	"testing"
	"time"
)

// setEnv sets env vars for a test, unsets those mapped to "", and restores all on cleanup.
// Uses os.LookupEnv so it can distinguish "unset" from "set to empty string".
func setEnv(t *testing.T, pairs map[string]string) {
	t.Helper()
	type saved struct {
		value   string
		present bool
	}
	originals := make(map[string]saved, len(pairs))
	for k, v := range pairs {
		val, ok := os.LookupEnv(k)
		originals[k] = saved{value: val, present: ok}
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
	t.Cleanup(func() {
		for k, orig := range originals {
			if !orig.present {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, orig.value)
			}
		}
	})
}

func TestLoadAWSConfig_RegionFromEnv(t *testing.T) {
	setEnv(t, map[string]string{
		"AWS_REGION":            "us-west-2",
		"AWS_ACCESS_KEY_ID":     "AKIAIOSFODNN7EXAMPLE",
		"AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	})

	cfg, err := LoadAWSConfig(context.Background(), 0)
	if err != nil {
		t.Fatalf("LoadAWSConfig() unexpected error: %v", err)
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("cfg.Region = %q, want %q", cfg.Region, "us-west-2")
	}
}

func TestLoadAWSConfig_RegionFromEnvWithPositiveDialTimeout(t *testing.T) {
	// When AWS_REGION is set, IMDS is not consulted regardless of dialTimeout.
	// dialTimeout > 0 enables the IMDS branch, but AWS_REGION short-circuits it.
	// This test runs quickly because the IMDS call is never made.
	setEnv(t, map[string]string{
		"AWS_REGION":            "ap-southeast-1",
		"AWS_ACCESS_KEY_ID":     "AKIAIOSFODNN7EXAMPLE",
		"AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	})

	cfg, err := LoadAWSConfig(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("LoadAWSConfig() unexpected error: %v", err)
	}
	if cfg.Region != "ap-southeast-1" {
		t.Errorf("cfg.Region = %q, want %q", cfg.Region, "ap-southeast-1")
	}
}

func TestLoadAWSConfig_NoCredentials(t *testing.T) {
	// Clear all known credential sources to ensure the no-credentials path is exercised.
	// Container/IRSA credential sources must also be cleared to avoid flakiness on EC2/ECS.
	setEnv(t, map[string]string{
		"AWS_REGION":                             "us-east-1",
		"AWS_ACCESS_KEY_ID":                      "",
		"AWS_SECRET_ACCESS_KEY":                  "",
		"AWS_SESSION_TOKEN":                      "",
		"AWS_PROFILE":                            "nonexistent-profile-xyz",
		"AWS_CONFIG_FILE":                        os.DevNull,
		"AWS_SHARED_CREDENTIALS_FILE":            os.DevNull,
		"AWS_CONTAINER_CREDENTIALS_FULL_URI":     "",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI": "",
		"AWS_WEB_IDENTITY_TOKEN_FILE":            "",
	})

	_, err := LoadAWSConfig(context.Background(), 0)
	if err == nil {
		t.Fatal("LoadAWSConfig() expected error for missing credentials, got nil")
	}
}
