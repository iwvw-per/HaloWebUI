package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	ID        string `json:"id"`
	JWTID     string `json:"jti"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp,omitempty"`
}

func LoadOrCreateSecret(dataDir string) ([]byte, error) {
	if value := os.Getenv("WEBUI_SECRET_KEY"); value != "" {
		return []byte(value), nil
	}
	if value := os.Getenv("WEBUI_JWT_SECRET_KEY"); value != "" {
		return []byte(value), nil
	}
	path := filepath.Join(dataDir, ".webui_secret_key")
	secret, err := os.ReadFile(path)
	if err == nil && len(secret) >= 32 {
		return secret, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read webui secret: %w", err)
	}
	secret = make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate webui secret: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory for webui secret: %w", err)
	}
	encoded := make([]byte, hex.EncodedLen(len(secret)))
	hex.Encode(encoded, secret)
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return nil, fmt.Errorf("persist webui secret: %w", err)
	}
	return encoded, nil
}

func HashPassword(password string) (string, error) {
	if len([]byte(password)) > 72 {
		return "", errors.New("password exceeds bcrypt's 72-byte limit")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(hash), err
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func NewToken(secret []byte, userID string, expiresAfter time.Duration) (string, Claims, error) {
	now := time.Now().UTC()
	claims := Claims{
		ID:       userID,
		JWTID:    randomID(),
		IssuedAt: now.Unix(),
	}
	if expiresAfter > 0 {
		claims.ExpiresAt = now.Add(expiresAfter).Unix()
	}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", Claims{}, err
	}
	unsigned := rawBase64(header) + "." + rawBase64(payload)
	return unsigned + "." + rawBase64(sign(secret, unsigned)), claims, nil
}

func ParseToken(secret []byte, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	providedSignature, err := decodeCanonicalBase64(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	expectedSignature := sign(secret, parts[0]+"."+parts[1])
	if len(providedSignature) != len(expectedSignature) || subtle.ConstantTimeCompare(providedSignature, expectedSignature) != 1 {
		return Claims{}, ErrInvalidToken
	}
	headerJSON, err := decodeCanonicalBase64(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var header map[string]string
	if json.Unmarshal(headerJSON, &header) != nil || header["alg"] != "HS256" {
		return Claims{}, ErrInvalidToken
	}
	payload, err := decodeCanonicalBase64(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if json.Unmarshal(payload, &claims) != nil || claims.ID == "" || claims.JWTID == "" {
		return Claims{}, ErrInvalidToken
	}
	if claims.ExpiresAt > 0 && time.Now().Unix() >= claims.ExpiresAt {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func randomID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic("cryptographic random source unavailable")
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16],
	)
}

// RandomIDForInternalUse returns a UUID-shaped identifier for persisted records.
func RandomIDForInternalUse() string {
	return randomID()
}

func rawBase64(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

// decodeCanonicalBase64 rejects alternate encodings that decode to the same
// bytes. A JWT segment must round-trip to exactly the bytes presented by the
// caller; otherwise a one-character token mutation can remain valid.
func decodeCanonicalBase64(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || rawBase64(decoded) != value {
		return nil, ErrInvalidToken
	}
	return decoded, nil
}

func sign(secret []byte, value string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
