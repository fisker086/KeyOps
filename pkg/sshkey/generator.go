package sshkey

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// KeyPair represents an SSH key pair
type KeyPair struct {
	PrivateKey  string // OpenSSH format (modern format compatible with OpenSSH 7.8+)
	PublicKey   string // OpenSSH format
	Fingerprint string // SHA256 fingerprint
}

// GenerateRSAKeyPair generates a new RSA key pair for SSH authentication
// The private key is generated in OpenSSH format (compatible with OpenSSH 7.8+)
func GenerateRSAKeyPair(bitSize int) (*KeyPair, error) {
	if bitSize < 2048 {
		bitSize = 2048 // Minimum secure key size
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Encode private key to OpenSSH format (modern format)
	// This format is compatible with OpenSSH 7.8+ (2018 and later)
	privateKeyPEM, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Convert PEM block to bytes
	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)

	// Generate public key
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	// Format public key in OpenSSH format
	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)

	// Calculate fingerprint
	fingerprint := ssh.FingerprintSHA256(publicKey)

	return &KeyPair{
		PrivateKey:  string(privateKeyBytes),
		PublicKey:   string(publicKeyBytes),
		Fingerprint: fingerprint,
	}, nil
}

// ParsePublicKey parses an OpenSSH format public key
func ParsePublicKey(publicKeyStr string) (ssh.PublicKey, error) {
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	return publicKey, nil
}

// GetFingerprint calculates the SHA256 fingerprint of a public key
func GetFingerprint(publicKeyStr string) (string, error) {
	publicKey, err := ParsePublicKey(publicKeyStr)
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(publicKey), nil
}
