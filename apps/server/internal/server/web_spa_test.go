package server

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/lifei6671/plaindoc/apps/server/internal/config"
)

func TestLayeredHTTPFileSystem_OpenFallsBackToSecondaryDirectory(t *testing.T) {
	tempDir := t.TempDir()
	primaryDir := filepath.Join(tempDir, "primary")
	secondaryDir := filepath.Join(tempDir, "secondary")

	if err := os.MkdirAll(primaryDir, 0o755); err != nil {
		t.Fatalf("create primary dir: %v", err)
	}
	if err := os.MkdirAll(secondaryDir, 0o755); err != nil {
		t.Fatalf("create secondary dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(primaryDir, "from-primary.txt"), []byte("primary"), 0o644); err != nil {
		t.Fatalf("write primary file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondaryDir, "from-secondary.txt"), []byte("secondary"), 0o644); err != nil {
		t.Fatalf("write secondary file: %v", err)
	}

	layeredFS := newLayeredHTTPFileSystem([]string{primaryDir, secondaryDir})

	primaryFile, err := layeredFS.Open("/from-primary.txt")
	if err != nil {
		t.Fatalf("open primary file: %v", err)
	}
	primaryContent, err := io.ReadAll(primaryFile)
	_ = primaryFile.Close()
	if err != nil {
		t.Fatalf("read primary file: %v", err)
	}
	if string(primaryContent) != "primary" {
		t.Fatalf("expected primary content, got %q", string(primaryContent))
	}

	secondaryFile, err := layeredFS.Open("/from-secondary.txt")
	if err != nil {
		t.Fatalf("open secondary file: %v", err)
	}
	secondaryContent, err := io.ReadAll(secondaryFile)
	_ = secondaryFile.Close()
	if err != nil {
		t.Fatalf("read secondary file: %v", err)
	}
	if string(secondaryContent) != "secondary" {
		t.Fatalf("expected secondary content, got %q", string(secondaryContent))
	}

	if _, err := layeredFS.Open("/missing.txt"); err == nil {
		t.Fatal("expected error when opening missing file")
	}
}

func TestResolveWebAssetsDirs_ReturnsDistThenDistSSR(t *testing.T) {
	tempDir := t.TempDir()
	distDir := filepath.Join(tempDir, "apps", "web", "dist")
	distAssetsDir := filepath.Join(distDir, "web-assets")
	distSSRDir := filepath.Join(tempDir, "apps", "web", "dist-ssr")
	distSSRAssetsDir := filepath.Join(distSSRDir, "web-assets")

	if err := os.MkdirAll(distAssetsDir, 0o755); err != nil {
		t.Fatalf("create dist web-assets dir: %v", err)
	}
	if err := os.MkdirAll(distSSRAssetsDir, 0o755); err != nil {
		t.Fatalf("create dist-ssr web-assets dir: %v", err)
	}

	cfg := config.Config{
		SSRWorker: config.SSRWorkerConfig{
			Entry: filepath.Join(distSSRDir, "worker-entry.js"),
		},
	}

	dirs := resolveWebAssetsDirs(distDir, cfg)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 web-assets dirs, got %d (%v)", len(dirs), dirs)
	}
	if dirs[0] != distAssetsDir {
		t.Fatalf("expected first dir %q, got %q", distAssetsDir, dirs[0])
	}
	if dirs[1] != distSSRAssetsDir {
		t.Fatalf("expected second dir %q, got %q", distSSRAssetsDir, dirs[1])
	}
}
