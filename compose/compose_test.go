package compose

import (
	"slices"
	"testing"
)

func TestParseFile(t *testing.T) {
	f, err := ParseFile("testdata/docker-compose.yml")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(f.Services) != 4 {
		t.Fatalf("expected 4 services, got %d", len(f.Services))
	}

	t.Run("image service", func(t *testing.T) {
		db, ok := f.Services["db"]
		if !ok {
			t.Fatal("service 'db' not found")
		}
		if db.Image != "postgres:16" {
			t.Errorf("image: got %q, want %q", db.Image, "postgres:16")
		}
		if db.Environment["POSTGRES_USER"] != "user" {
			t.Errorf("env POSTGRES_USER: got %q", db.Environment["POSTGRES_USER"])
		}
		if db.Restart != "unless-stopped" {
			t.Errorf("restart: got %q", db.Restart)
		}
	})

	t.Run("ports", func(t *testing.T) {
		cache := f.Services["cache"]
		if len(cache.Ports) != 1 || cache.Ports[0] != "6379:6379" {
			t.Errorf("ports: got %v", cache.Ports)
		}
	})

	t.Run("build map form", func(t *testing.T) {
		api := f.Services["api"]
		if api.Build == nil {
			t.Fatal("expected build config")
		}
		if api.Build.Context != "./api" {
			t.Errorf("build context: got %q", api.Build.Context)
		}
	})

	t.Run("depends_on map form", func(t *testing.T) {
		api := f.Services["api"]
		if len(api.DependsOn) != 2 {
			t.Fatalf("depends_on: got %d entries", len(api.DependsOn))
		}
		if !slices.Contains(api.DependsOn, "db") || !slices.Contains(api.DependsOn, "cache") {
			t.Errorf("depends_on: got %v", api.DependsOn)
		}
	})

	t.Run("depends_on list form", func(t *testing.T) {
		worker := f.Services["worker"]
		if len(worker.DependsOn) != 2 {
			t.Fatalf("depends_on: got %d entries", len(worker.DependsOn))
		}
		if !slices.Contains(worker.DependsOn, "db") || !slices.Contains(worker.DependsOn, "cache") {
			t.Errorf("depends_on: got %v", worker.DependsOn)
		}
	})
}

func TestParseNoServices(t *testing.T) {
	_, err := Parse([]byte("version: '3'\n"))
	if err == nil {
		t.Fatal("expected error for file with no services")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	_, err := Parse([]byte(":\t invalid yaml\n"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
