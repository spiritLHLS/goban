package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/spiritlhl/goban/internal/config"
)

const encryptedPrefix = "enc:v1:"

// EncryptString encrypts sensitive values before they are persisted.
func EncryptString(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}

	block, err := aes.NewCipher(secretKey())
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	cipherText := gcm.Seal(nil, nonce, []byte(plainText), nil)
	payload := append(nonce, cipherText...)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

// DecryptString decrypts values produced by EncryptString. Plain values are
// returned as-is so manually reset databases and older local files remain usable.
func DecryptString(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, encryptedPrefix) {
		return value, nil
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("解码密文失败: %w", err)
	}

	block, err := aes.NewCipher(secretKey())
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("密文长度无效")
	}

	nonce := raw[:gcm.NonceSize()]
	cipherText := raw[gcm.NonceSize():]
	plainText, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败，请检查 GOBAN_SECRET_KEY 是否变化: %w", err)
	}

	return string(plainText), nil
}

func secretKey() []byte {
	sum := sha256.Sum256([]byte(config.GetConfig().SecretKey))
	return sum[:]
}
