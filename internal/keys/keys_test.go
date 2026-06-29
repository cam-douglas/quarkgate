package keys

import "testing"

func TestGenerateAndCompare(t *testing.T) {
	full, prefix, hash, err := Generate(false)
	if err != nil {
		t.Fatal(err)
	}
	if !hasPrefix(full, LivePrefix) {
		t.Fatal("bad prefix")
	}
	if len(prefix) < 8 {
		t.Fatal("short prefix")
	}
	if !Compare(hash, full) {
		t.Fatal("compare failed")
	}
}

func TestValidateFormat(t *testing.T) {
	if err := ValidateFormat("invalid"); err == nil {
		t.Fatal("expected error")
	}
	full, _, _, _ := Generate(true)
	if err := ValidateFormat(full); err != nil {
		t.Fatal(err)
	}
}
