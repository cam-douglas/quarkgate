package vault

import "testing"

func TestEncryptDecrypt(t *testing.T) {
	v, err := New("test-key-32-bytes-long-enough!!")
	if err != nil {
		t.Fatal(err)
	}
	plain := "sk-secret-api-key"
	blob, err := v.Encrypt([]byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	out, err := v.Decrypt(blob)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != plain {
		t.Fatalf("got %s", string(out))
	}
}

func TestEncryptStringRoundTrip(t *testing.T) {
	v, err := New("another-test-key-32-bytes!!!!")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := v.EncryptString("hello")
	if err != nil {
		t.Fatal(err)
	}
	dec, err := v.DecryptString(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != "hello" {
		t.Fatalf("got %s", dec)
	}
}
