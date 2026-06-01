package app

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultWASafeServerPublicKeyHex = "8e8c0f74c3ebc5d7a6865c6c3c843856b06121cce8ea774d22fb6f122512302d"

type nativeHTTPClient struct {
	client *http.Client
}

func newNativeHTTPClient(proxy string) (*nativeHTTPClient, error) {
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	if proxy != "" {
		parsed, err := proxyURL(proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &nativeHTTPClient{client: &http.Client{Timeout: 20 * time.Second, Transport: transport}}, nil
}

func (c *nativeHTTPClient) postWASafe(ctx context.Context, endpoint string, plain string, userAgent string) (map[string]any, string, error) {
	if endpoint == "" {
		return nil, "", fmt.Errorf("endpoint is not configured")
	}
	enc, err := encryptWASafe([]byte(plain), defaultWASafeServerPublicKeyHex)
	if err != nil {
		return nil, "", err
	}
	body := "ENC=" + enc
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", firstNonEmpty(userAgent, nativeUserAgent(defaultWAAppVersion)))
	req.Header.Set("WaMsysRequest", "1")
	req.Header.Set("X-Forwarded-Host", defaultNativeHTTPHost)
	req.Header.Set("request_token", strings.ToUpper(newUUIDString()))
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, enc, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	result := map[string]any{"status_code": float64(resp.StatusCode), "response_text": string(data)}
	var parsed map[string]any
	if json.Unmarshal(data, &parsed) == nil {
		for key, value := range parsed {
			result[key] = value
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, enc, fmt.Errorf("wasafe endpoint returned status %d", resp.StatusCode)
	}
	return result, enc, nil
}

func encryptWASafe(plaintext []byte, serverPublicKeyHex string) (string, error) {
	serverRaw, err := hex.DecodeString(serverPublicKeyHex)
	if err != nil {
		return "", err
	}
	serverPublic, err := ecdh.X25519().NewPublicKey(serverRaw)
	if err != nil {
		return "", err
	}
	ephemeral, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	shared, err := ephemeral.ECDH(serverPublic)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(shared)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, make([]byte, 12), plaintext, nil)
	combined := append(append([]byte{}, ephemeral.PublicKey().Bytes()...), sealed...)
	return b64u(combined), nil
}

func encHash(enc string) string {
	sum := sha256.Sum256([]byte(enc))
	return hex.EncodeToString(sum[:])
}
