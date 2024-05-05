package hypatia

import (
	"path/filepath"
	"testing"
)

func TestLocalHealthCheck(t *testing.T) {
	testDir := t.TempDir()
	path := filepath.Join(testDir, "test")
	hc := FileHealthcheck{
		Filepath: path,
	}
	if err := hc.GetHealth(); err == nil {
		t.Fatal("expected test to fail first healthcheck: ", err)
	}
	if err := hc.SetHealth(true); err != nil {
		t.Fatal("expected set health to work correctly: ", err)
	}
	if err := hc.GetHealth(); err != nil {
		t.Fatal("expected health to be updated to succeed: ", err)
	}
	if err := hc.SetHealth(false); err != nil {
		t.Fatal("expected set health to work correctly (part 2): ", err)
	}
	if err := hc.GetHealth(); err == nil {
		t.Fatal("expected test to fail final healthcheck: ", err)
	}
}
