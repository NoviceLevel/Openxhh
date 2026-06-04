package config

import "testing"

func TestCreateDefaultConfigReturnsWriteError(t *testing.T) {
	oldConfig := ConfigStruct
	t.Cleanup(func() {
		ConfigStruct = oldConfig
	})

	if err := createDefaultConfig(t.TempDir()); err == nil {
		t.Fatal("createDefaultConfig returned nil error for directory path")
	}
}
