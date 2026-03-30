package connector

import (
	"os"
	"path/filepath"
	"testing"

	"openpaws/internal/model"
)

func TestFixtureLoaderLoadsAccounts(t *testing.T) {
	t.Parallel()
	loader := FixtureLoader{}
	accounts, err := loader.Load(filepath.Join("..", "..", "testdata", "fixtures"), []model.Platform{model.PlatformInstagram, model.PlatformX})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(accounts) != 5 {
		t.Fatalf("expected 5 accounts, got %d", len(accounts))
	}
}

func TestFixtureLoaderFiltersByPlatform(t *testing.T) {
	t.Parallel()
	loader := FixtureLoader{}
	accounts, err := loader.Load(filepath.Join("..", "..", "testdata", "fixtures"), []model.Platform{model.PlatformInstagram})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, a := range accounts {
		if a.Platform != model.PlatformInstagram {
			t.Fatalf("expected only instagram accounts, got %s", a.Platform)
		}
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 instagram accounts, got %d", len(accounts))
	}
}

func TestFixtureLoaderMissingDir(t *testing.T) {
	t.Parallel()
	loader := FixtureLoader{}
	_, err := loader.Load("/nonexistent/path", []model.Platform{model.PlatformInstagram})
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestFixtureLoaderEmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	loader := FixtureLoader{}
	_, err := loader.Load(dir, []model.Platform{model.PlatformInstagram})
	if err == nil {
		t.Fatal("expected error for empty fixture directory")
	}
}

func TestFixtureLoaderInvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	loader := FixtureLoader{}
	_, err = loader.Load(dir, []model.Platform{model.PlatformInstagram, model.PlatformX})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
