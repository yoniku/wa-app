package app

import (
	"strings"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

func (e *NativeEngine) httpForProxy() (*nativeHTTPClient, error) {
	if _, err := e.proxyURL(); err != nil {
		return nil, err
	}
	return e.http, nil
}

func (e *NativeEngine) proxyURL() (string, error) {
	if proxyURL := strings.TrimSpace(e.cfg.ProxyURL); proxyURL != "" {
		return proxyURL, nil
	}
	return "", NewError(waappv1.WaErrorCode_WA_ERROR_CODE_VALIDATION_FAILED, "proxy_url is required; WA runtime must use a proxy-runtime US dynamic IP lease or WA_APP_PROXY_URL", false)
}
