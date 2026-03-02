package search

import (
	"strings"
	"testing"
)

func TestValidateConfigPayload_Success(t *testing.T) {
	payload := map[string]any{
		"enabled":        true,
		"activeProvider": "bleve",
		"fallbackPolicy": "degrade_to_bleve",
		"analysis": map[string]any{
			"activeAnalyzer": "jieba",
			"analyzers": map[string]any{
				"simple": map[string]any{
					"enabled": true,
				},
				"jieba": map[string]any{
					"enabled":          true,
					"mode":             "search",
					"hmm":              true,
					"stopwordsEnabled": false,
					"dictSource":       "db",
					"dictVersion":      "v2026-03-02-001",
				},
			},
		},
	}

	if err := ValidateConfigPayload(payload); err != nil {
		t.Fatalf("expected valid payload, got err=%v", err)
	}
}

func TestValidateConfigPayload_ActiveAnalyzerMustBeEnabled(t *testing.T) {
	payload := map[string]any{
		"enabled":        true,
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

	err := ValidateConfigPayload(payload)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "activeAnalyzer jieba must be enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigPayload_RejectsInvalidDictVersion(t *testing.T) {
	payload := map[string]any{
		"enabled":        true,
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
					"dictVersion":      "invalid version",
				},
			},
		},
	}

	err := ValidateConfigPayload(payload)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "dictVersion is invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeConfig_UsesDefaults(t *testing.T) {
	config := NormalizeConfig(map[string]any{
		"enabled": true,
		"analysis": map[string]any{
			"analyzers": map[string]any{
				"jieba": map[string]any{
					"enabled":     true,
					"dictVersion": "v2",
				},
			},
		},
	})

	if config.ActiveProvider != ProviderBleve {
		t.Fatalf("expected default provider %q, got %q", ProviderBleve, config.ActiveProvider)
	}
	if !config.Enabled {
		t.Fatalf("expected enabled=true from payload")
	}
	if config.Analysis.ActiveAnalyzer != AnalyzerSimple {
		t.Fatalf("expected default active analyzer %q, got %q", AnalyzerSimple, config.Analysis.ActiveAnalyzer)
	}
	if !config.Analysis.Analyzers.Jieba.Enabled {
		t.Fatalf("expected jieba enabled from payload")
	}
	if config.Analysis.Analyzers.Jieba.DictVersion != "v2" {
		t.Fatalf("expected jieba dict version v2, got %q", config.Analysis.Analyzers.Jieba.DictVersion)
	}
	if config.Analysis.Analyzers.Jieba.Mode != JiebaModeSearch {
		t.Fatalf("expected default jieba mode %q, got %q", JiebaModeSearch, config.Analysis.Analyzers.Jieba.Mode)
	}
}

func TestValidateConfigPayload_SupportsDatabaseProvider(t *testing.T) {
	payload := map[string]any{
		"enabled":        true,
		"activeProvider": "database",
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
					"dictVersion":      "default",
				},
			},
		},
	}
	if err := ValidateConfigPayload(payload); err != nil {
		t.Fatalf("expected database provider valid, got err=%v", err)
	}
}
