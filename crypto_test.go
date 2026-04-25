package main

import (
	"os"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("Expected 32-byte key, got %d bytes", len(key))
	}

	// Two keys should be different (randomness check)
	key2, _ := GenerateKey()
	if string(key) == string(key2) {
		t.Error("Two generated keys should not be identical")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key, _ := GenerateKey()

	tests := []struct {
		name      string
		plaintext string
	}{
		{"Simple text", "hello world"},
		{"JSON object", `{"email":"test@example.com","password":"secret123"}`},
		{"Empty string", ""},
		{"Unicode", "München, Österreich — straße 日本語"},
		{"Large text", string(make([]byte, 10000))},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := Encrypt(key, []byte(tc.plaintext))
			if err != nil {
				t.Fatalf("Encrypt() error: %v", err)
			}

			// Encrypted should be different from plaintext
			if tc.plaintext != "" && string(encrypted) == tc.plaintext {
				t.Error("Encrypted text should not equal plaintext")
			}

			decrypted, err := Decrypt(key, encrypted)
			if err != nil {
				t.Fatalf("Decrypt() error: %v", err)
			}

			if string(decrypted) != tc.plaintext {
				t.Errorf("Round-trip failed: got %q, want %q", string(decrypted), tc.plaintext)
			}
		})
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()

	encrypted, _ := Encrypt(key1, []byte("secret data"))

	_, err := Decrypt(key2, encrypted)
	if err == nil {
		t.Error("Decrypt with wrong key should fail")
	}
}

func TestDecrypt_CorruptedData(t *testing.T) {
	key, _ := GenerateKey()

	// Too short
	_, err := Decrypt(key, []byte("short"))
	if err == nil {
		t.Error("Decrypt of too-short data should fail")
	}

	// Corrupted ciphertext
	encrypted, _ := Encrypt(key, []byte("test"))
	encrypted[len(encrypted)-1] ^= 0xFF // Flip bits in last byte
	_, err = Decrypt(key, encrypted)
	if err == nil {
		t.Error("Decrypt of corrupted data should fail")
	}
}

func TestEncryptedJSON_RoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	tmpFile := "test_encrypted_roundtrip.enc"
	defer os.Remove(tmpFile)

	type testData struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	original := testData{Email: "user@example.com", Password: "P@ssw0rd!"}

	// Write
	err := WriteEncryptedJSON(key, tmpFile, original)
	if err != nil {
		t.Fatalf("WriteEncryptedJSON() error: %v", err)
	}

	// Verify file exists and is not plaintext
	raw, _ := os.ReadFile(tmpFile)
	if string(raw) == `{"email":"user@example.com","password":"P@ssw0rd!"}` {
		t.Error("File should be encrypted, not plaintext")
	}

	// Read back
	var loaded testData
	found, err := ReadEncryptedJSON(key, tmpFile, &loaded)
	if err != nil {
		t.Fatalf("ReadEncryptedJSON() error: %v", err)
	}
	if !found {
		t.Fatal("ReadEncryptedJSON() returned false, expected true")
	}
	if loaded.Email != original.Email || loaded.Password != original.Password {
		t.Errorf("Data mismatch: got %+v, want %+v", loaded, original)
	}
}

func TestReadEncryptedJSON_FileNotFound(t *testing.T) {
	key, _ := GenerateKey()
	var data map[string]string
	found, err := ReadEncryptedJSON(key, "nonexistent_file.enc", &data)
	if err != nil {
		t.Fatalf("Should not error on missing file: %v", err)
	}
	if found {
		t.Error("Should return false for missing file")
	}
}

func TestLoadOrCreateKey(t *testing.T) {
	tmpFile := "test_key_file.key"
	defer os.Remove(tmpFile)

	// First call: creates key
	key1, err := LoadOrCreateKey(tmpFile)
	if err != nil {
		t.Fatalf("LoadOrCreateKey() error: %v", err)
	}
	if len(key1) != 32 {
		t.Errorf("Expected 32-byte key, got %d", len(key1))
	}

	// Second call: loads same key
	key2, err := LoadOrCreateKey(tmpFile)
	if err != nil {
		t.Fatalf("LoadOrCreateKey() second call error: %v", err)
	}
	if string(key1) != string(key2) {
		t.Error("Key should be the same on second load")
	}
}

func TestMigrateToEncrypted(t *testing.T) {
	key, _ := GenerateKey()
	plaintextFile := "test_migrate_plain.json"
	encryptedFile := "test_migrate_plain.enc"
	defer os.Remove(plaintextFile)
	defer os.Remove(encryptedFile)

	// Write plaintext JSON
	os.WriteFile(plaintextFile, []byte(`{"key":"value"}`), 0644)

	// Migrate
	err := MigrateToEncrypted(key, plaintextFile, encryptedFile)
	if err != nil {
		t.Fatalf("MigrateToEncrypted() error: %v", err)
	}

	// Plaintext should be deleted
	if _, err := os.Stat(plaintextFile); !os.IsNotExist(err) {
		t.Error("Plaintext file should have been deleted")
	}

	// Encrypted file should exist and be readable
	var data map[string]string
	found, err := ReadEncryptedJSON(key, encryptedFile, &data)
	if err != nil {
		t.Fatalf("ReadEncryptedJSON() after migration error: %v", err)
	}
	if !found {
		t.Fatal("Encrypted file should exist after migration")
	}
	if data["key"] != "value" {
		t.Errorf("Migrated data mismatch: got %v", data)
	}

	// Second call should be no-op
	err = MigrateToEncrypted(key, plaintextFile, encryptedFile)
	if err != nil {
		t.Fatalf("Second MigrateToEncrypted() should be no-op: %v", err)
	}
}
