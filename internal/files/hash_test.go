package files

import "testing"

func TestParseLockFileBody(t *testing.T) {
	got, err := ParseLockFileBody([]byte(".env: abc123\nconfig/secrets.yaml: def456\n"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got[".env"] != "abc123" || got["config/secrets.yaml"] != "def456" {
		t.Fatalf("unexpected hashes: %#v", got)
	}
}

func TestParseLockFileBodyRejectsInvalidLine(t *testing.T) {
	_, err := ParseLockFileBody([]byte("invalid\n"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestHashesEqual(t *testing.T) {
	a := map[string]string{".env": "abc123"}
	b := map[string]string{".env": "abc123"}
	c := map[string]string{".env": "changed"}

	if !HashesEqual(a, b) {
		t.Fatal("expected hashes to be equal")
	}
	if HashesEqual(a, c) {
		t.Fatal("expected hashes to differ")
	}
}
