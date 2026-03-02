package service

import "testing"

func TestResolveSystemConfigValidator_Search(t *testing.T) {
	configKey, validator, err := resolveSystemConfigValidator("search")
	if err != nil {
		t.Fatalf("resolve search validator failed: %v", err)
	}
	if configKey != "search" {
		t.Fatalf("expected config key search, got %q", configKey)
	}
	if validator == nil {
		t.Fatal("expected non-nil search validator")
	}
}

func TestValidateSearchConfig(t *testing.T) {
	valid := map[string]any{
		"activeProvider": "bleve",
		"fallbackPolicy": "degrade_to_bleve",
		"analysis": map[string]any{
			"activeAnalyzer": "simple",
			"analyzers": map[string]any{
				"simple": map[string]any{
					"enabled": true,
				},
				"jieba": map[string]any{
					"enabled":          false,
					"mode":             "search",
					"hmm":              true,
					"stopwordsEnabled": false,
					"dictSource":       "db",
					"dictVersion":      "v2026-03-02-001",
				},
			},
		},
	}
	if err := validateSearchConfig(valid); err != nil {
		t.Fatalf("expected valid search config, got err=%v", err)
	}

	invalid := map[string]any{
		"activeProvider": "bleve",
		"fallbackPolicy": "degrade_to_bleve",
		"analysis": map[string]any{
			"activeAnalyzer": "jieba",
			"analyzers": map[string]any{
				"simple": map[string]any{
					"enabled": true,
				},
				"jieba": map[string]any{
					"enabled":          false,
					"mode":             "search",
					"hmm":              true,
					"stopwordsEnabled": false,
					"dictSource":       "db",
					"dictVersion":      "v2026-03-02-001",
				},
			},
		},
	}
	if err := validateSearchConfig(invalid); err == nil {
		t.Fatal("expected invalid search config error")
	}
}
