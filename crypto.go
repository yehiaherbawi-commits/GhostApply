package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ============================================================================
// ENCRYPTION VAULT — AES-256-GCM for Secrets at Rest
// ============================================================================

// GenerateKey creates a cryptographically secure 32-byte key for AES-256.
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %v", err)
	}
	return key, nil
}

// LoadOrCreateKey reads the encryption key from a file, or generates and
// saves a new one if the file doesn't exist. The key file is created with
// restrictive permissions (0600).
func LoadOrCreateKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil && len(data) == 32 {
		return data, nil
	}

	// Key file missing or corrupted — generate a new one
	key, err := GenerateKey()
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to save key file: %v", err)
	}

	fmt.Printf("🔑 [Security] Generated new encryption key: %s\n", path)
	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// The nonce is prepended to the ciphertext for self-contained decryption.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher error: %v", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM error: %v", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce error: %v", err)
	}

	// Seal appends the encrypted+authenticated ciphertext to the nonce
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext that was produced by Encrypt.
// It expects the nonce to be prepended to the ciphertext.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cipher error: %v", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("GCM error: %v", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBody := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBody, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (wrong key or corrupted data): %v", err)
	}

	return plaintext, nil
}

// ReadEncryptedJSON reads an encrypted file, decrypts it, and unmarshals the
// JSON content into the provided target. If the file doesn't exist, it returns
// false (no error) so callers can initialize defaults.
func ReadEncryptedJSON(key []byte, path string, target interface{}) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // File doesn't exist yet
		}
		return false, fmt.Errorf("read error: %v", err)
	}

	plaintext, err := Decrypt(key, data)
	if err != nil {
		return false, fmt.Errorf("decrypt error for %s: %v", path, err)
	}

	if err := json.Unmarshal(plaintext, target); err != nil {
		return false, fmt.Errorf("JSON parse error for %s: %v", path, err)
	}

	return true, nil
}

// WriteEncryptedJSON marshals the data to JSON, encrypts it, and writes it
// to the specified file with restrictive permissions (0600).
func WriteEncryptedJSON(key []byte, path string, data interface{}) error {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON marshal error: %v", err)
	}

	encrypted, err := Encrypt(key, jsonBytes)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		return fmt.Errorf("write error for %s: %v", path, err)
	}

	return nil
}

// MigrateToEncrypted checks if a plaintext JSON file exists and, if so,
// encrypts it to a new .enc file and deletes the plaintext original.
// This is idempotent: if the .enc file already exists, it does nothing.
func MigrateToEncrypted(key []byte, plaintextPath, encryptedPath string) error {
	// If encrypted file already exists, nothing to do
	if _, err := os.Stat(encryptedPath); err == nil {
		return nil
	}

	// Check if plaintext file exists
	data, err := os.ReadFile(plaintextPath)
	if err != nil {
		return nil // No plaintext file either — nothing to migrate
	}

	// Validate it's valid JSON before encrypting
	var check interface{}
	if err := json.Unmarshal(data, &check); err != nil {
		return fmt.Errorf("plaintext file %s is not valid JSON: %v", plaintextPath, err)
	}

	// Encrypt and write
	encrypted, err := Encrypt(key, data)
	if err != nil {
		return fmt.Errorf("encryption failed during migration: %v", err)
	}

	if err := os.WriteFile(encryptedPath, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %v", err)
	}

	// Delete plaintext
	if err := os.Remove(plaintextPath); err != nil {
		fmt.Printf("   ⚠️  Could not delete plaintext file %s: %v\n", plaintextPath, err)
		fmt.Println("   💡 Please delete it manually for security.")
	} else {
		fmt.Printf("🔒 [Security] Migrated %s → %s (plaintext deleted)\n", plaintextPath, encryptedPath)
	}

	return nil
}
