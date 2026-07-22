package database

import "testing"

func TestMigrationChecksumIsStableAndContentSensitive(t *testing.T) {
	t.Parallel()
	first := migrationChecksum([]byte("SELECT 1;\n"))
	if first != migrationChecksum([]byte("SELECT 1;\n")) {
		t.Fatal("checksum changed for identical migration content")
	}
	if first == migrationChecksum([]byte("SELECT 2;\n")) {
		t.Fatal("checksum did not change with migration content")
	}
	if len(first) != len("sha256:")+64 {
		t.Fatalf("unexpected checksum format %q", first)
	}
}
