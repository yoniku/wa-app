package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
	"github.com/byte-v-forge/wa-app/internal/app"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type dashboardHTTP struct {
	staticDir        string
	n8nWebhookBase   string
	proxyRuntimeBase string
	client           *http.Client
	service          *app.Server
	actionHandler    http.Handler
}

func runDashboardHTTP(ctx context.Context, listenAddr, staticDir, n8nWebhookBase string, proxyRuntimeBase string, service *app.Server, actionHandler http.Handler) error {
	if strings.TrimSpace(listenAddr) == "" {
		return nil
	}
	server := &dashboardHTTP{
		staticDir:        firstNonEmpty(staticDir, "/app/dashboard/wa"),
		n8nWebhookBase:   strings.TrimRight(strings.TrimSpace(n8nWebhookBase), "/"),
		proxyRuntimeBase: strings.TrimRight(strings.TrimSpace(proxyRuntimeBase), "/"),
		client:           &http.Client{Timeout: 7 * time.Minute},
		service:          service,
		actionHandler:    actionHandler,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/wa/health", server.handleHealth)
	mux.HandleFunc("/api/wa/number-sms-probe", server.handleProbe)
	mux.HandleFunc("/api/wa/register", server.handleRegister)
	mux.HandleFunc("/api/wa/login-state-check", server.handleLoginStateCheck)
	mux.HandleFunc("/api/wa/accounts", server.handleAccounts)
	mux.HandleFunc("/api/wa/long-connections", server.handleLongConnections)
	mux.Handle("/api/wa/actions/", server.actionHandler)
	mux.Handle("/mf/wa/", http.StripPrefix("/mf/wa/", noCacheFileServer(server.staticDir)))
	mux.HandleFunc("/healthz", server.handleHealth)
	httpServer := &http.Server{Addr: listenAddr, Handler: withCORS(mux), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	log.Printf("wa-app dashboard BFF listening on %s", listenAddr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("wa-app dashboard BFF failed: %w", err)
	}
	return nil
}

func (s *dashboardHTTP) handleLongConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if s.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "wa-app service is not configured"})
		return
	}
	q := r.URL.Query()
	resp, err := s.service.GetLongConnectionStatus(r.Context(), &waappv1.GetLongConnectionStatusRequest{
		Context:              &waappv1.RequestContext{WorkspaceId: firstNonEmpty(q.Get("workspace_id"), "default"), RequestId: newRequestID("wa-conn-status")},
		LoginStateId:         q.Get("login_state_id"),
		WaAccountId:          q.Get("wa_account_id"),
		ClientProfileId:      q.Get("client_profile_id"),
		RegisteredIdentityId: q.Get("registered_identity_id"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load long connection status failed"})
		return
	}
	writeProtoJSON(w, http.StatusOK, resp)
}

func (s *dashboardHTTP) handleAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if s.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "wa-app service is not configured"})
		return
	}
	q := r.URL.Query()
	resp, err := s.service.ListWAAccounts(r.Context(), &waappv1.ListWAAccountsRequest{
		Context: &waappv1.RequestContext{WorkspaceId: firstNonEmpty(q.Get("workspace_id"), "default"), RequestId: newRequestID("wa-account-list")},
		Limit:   int32(positiveInt(q.Get("limit"), 100)),
		Cursor:  q.Get("cursor"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load WA accounts failed"})
		return
	}
	writeProtoJSON(w, http.StatusOK, resp)
}

func newWAActionHandler(service *app.Server) http.Handler {
	return app.NewActionGateway(service)
}

func (s *dashboardHTTP) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                     true,
		"n8n_webhook_configured": s.n8nWebhookBase != "",
		"workflows": []map[string]string{
			{"key": "number-sms-probe", "label": "WA 号码/SMS 检测", "webhook_path": "wa/number-sms-probe"},
			{"key": "register", "label": "WA 注册", "webhook_path": "wa/register"},
		},
	})
}

func (s *dashboardHTTP) handleProbe(w http.ResponseWriter, r *http.Request) {
	s.forwardWorkflow(w, r, "wa/number-sms-probe")
}

func (s *dashboardHTTP) handleRegister(w http.ResponseWriter, r *http.Request) {
	s.forwardWorkflow(w, r, "wa/register")
}

func (s *dashboardHTTP) handleLoginStateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	payload := map[string]any{}
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must be json"})
			return
		}
	}
	payload["workspace_id"] = firstNonEmpty(textField(payload, "workspace_id"), "default")
	payload["request_id"] = firstNonEmpty(textField(payload, "request_id"), newRequestID("wa-req"))
	payload["job_id"] = firstNonEmpty(textField(payload, "job_id"), newRequestID("wa-login-state-check"))
	if textField(payload, "proxy_url") == "" && textField(objectField(payload, "proxy"), "proxy_url") == "" && s.proxyRuntimeBase != "" {
		lease, err := s.acquireUSDynamicProxy(r.Context(), payload)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"success": false, "status": "US_DYNAMIC_IP_UNAVAILABLE", "error_message": "US random dynamic IP unavailable", "proxy": map[string]string{"proxy_mode": "US_RANDOM_DYNAMIC_IP", "country_code": "US"}})
			return
		}
		defer s.releaseDynamicProxy(context.Background(), lease.accountID)
		payload["proxy_url"] = lease.proxyURL
		payload["proxy"] = map[string]any{"proxy_mode": "US_RANDOM_DYNAMIC_IP", "country_code": "US", "account_id": lease.accountID, "lease_id": lease.leaseID}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "build login-state check request failed"})
		return
	}
	r.URL.Path = "/api/wa/actions/registration/check-login-state"
	r.Body = io.NopCloser(bytes.NewReader(encoded))
	r.ContentLength = int64(len(encoded))
	r.Header.Set("Content-Type", "application/json")
	s.actionHandler.ServeHTTP(w, r)
}

type dynamicProxyLease struct {
	accountID string
	leaseID   string
	proxyURL  string
}

func (s *dashboardHTTP) acquireUSDynamicProxy(ctx context.Context, payload map[string]any) (dynamicProxyLease, error) {
	endpoint, err := proxyRuntimeURL(s.proxyRuntimeBase, "/leases/acquire")
	if err != nil {
		return dynamicProxyLease{}, err
	}
	accountID := firstNonEmpty(textField(payload, "proxy_account_id"), "wa-login-check-"+newRequestID("lease"))
	requestBody := map[string]any{
		"account_id": accountID,
		"purpose":    "WA_LOGIN_STATE_CHECK",
		"force_new":  true,
		"policy": map[string]any{
			"mode":       "PROXY_SESSION_MODE_STICKY",
			"region":     "US",
			"sticky_ttl": "600s",
			"labels": map[string]string{
				"country_code": "US",
				"job_id":       textField(payload, "job_id"),
			},
		},
		"chain_policy": map[string]any{
			"country_code":                 "US",
			"purpose":                      "WA_LOGIN_STATE_CHECK",
			"strategy":                     "PROXY_CHAIN_STRATEGY_REGION_AWARE",
			"max_attempts":                 1,
			"allow_direct_dynamic_gateway": true,
			"prefer_line_proxy":            false,
		},
	}
	data, _ := json.Marshal(requestBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return dynamicProxyLease{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return dynamicProxyLease{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return dynamicProxyLease{}, fmt.Errorf("proxy-runtime acquire returned HTTP %d", resp.StatusCode)
	}
	raw := map[string]any{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return dynamicProxyLease{}, err
	}
	lease := objectField(raw, "lease")
	egress := objectField(raw, "egress")
	if len(egress) == 0 {
		egress = objectField(lease, "egress")
	}
	proxyURL, err := proxyURLFromEndpoint(s.proxyRuntimeBase, egress)
	if err != nil {
		return dynamicProxyLease{}, err
	}
	return dynamicProxyLease{accountID: firstNonEmpty(textField(lease, "account_id"), accountID), leaseID: textField(lease, "lease_id"), proxyURL: proxyURL}, nil
}

func (s *dashboardHTTP) releaseDynamicProxy(ctx context.Context, accountID string) {
	if strings.TrimSpace(accountID) == "" {
		return
	}
	endpoint, err := proxyRuntimeURL(s.proxyRuntimeBase, "/leases/release")
	if err != nil {
		return
	}
	data, _ := json.Marshal(map[string]string{"account_id": accountID})
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err == nil && resp != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		_ = resp.Body.Close()
	}
}

func (s *dashboardHTTP) forwardWorkflow(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if s.n8nWebhookBase == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "WA_N8N_WEBHOOK_BASE_URL is required"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	normalized, err := normalizeWorkflowBody(body, path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	url := s.n8nWebhookBase + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(normalized))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "build workflow request failed"})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "workflow request failed"})
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header, "Content-Type")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

var nonDigits = regexp.MustCompile(`\D+`)

var countryByInput = map[string]struct {
	iso string
	cc  string
}{
	"US": {"US", "1"}, "1": {"US", "1"}, "+1": {"US", "1"},
	"ID": {"ID", "62"}, "62": {"ID", "62"}, "+62": {"ID", "62"},
	"IN": {"IN", "91"}, "91": {"IN", "91"}, "+91": {"IN", "91"},
	"PH": {"PH", "63"}, "63": {"PH", "63"}, "+63": {"PH", "63"},
	"VN": {"VN", "84"}, "84": {"VN", "84"}, "+84": {"VN", "84"},
	"TH": {"TH", "66"}, "66": {"TH", "66"}, "+66": {"TH", "66"},
	"GB": {"GB", "44"}, "44": {"GB", "44"}, "+44": {"GB", "44"},
	"BR": {"BR", "55"}, "55": {"BR", "55"}, "+55": {"BR", "55"},
}

func normalizeWorkflowBody(body []byte, workflowPath string) ([]byte, error) {
	payload := map[string]any{}
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("request body must be json")
		}
	}
	workspaceID := textField(payload, "workspace_id")
	if workspaceID == "" {
		workspaceID = "default"
	}
	phoneObj, _ := payload["phone"].(map[string]any)
	regionInput := firstNonEmpty(textField(payload, "region"), textField(payload, "country_region"), textField(payload, "country_iso2"), textField(payload, "country_code"), textField(phoneObj, "country_iso2"), textField(phoneObj, "country_calling_code"), "US")
	country := resolveCountry(regionInput)
	cc := firstNonEmpty(textField(payload, "country_calling_code"), textField(payload, "cc"), textField(phoneObj, "country_calling_code"), country.cc)
	cc = strings.TrimPrefix(cc, "+")
	iso := strings.ToUpper(firstNonEmpty(textField(payload, "country_iso2"), textField(phoneObj, "country_iso2"), country.iso, "US"))
	rawNumber := firstNonEmpty(textField(payload, "national_number"), textField(payload, "number"), textField(payload, "phone_number"), textField(payload, "phone"), textField(phoneObj, "national_number"))
	national := nonDigits.ReplaceAllString(rawNumber, "")
	e164 := firstNonEmpty(textField(payload, "e164_number"), textField(phoneObj, "e164_number"))
	if e164 == "" && strings.HasPrefix(strings.TrimSpace(rawNumber), "+") {
		digits := nonDigits.ReplaceAllString(rawNumber, "")
		if inferred := inferCountryFromE164Digits(digits); inferred.cc != "" {
			cc = inferred.cc
			iso = inferred.iso
			national = strings.TrimPrefix(digits, cc)
		}
		e164 = "+" + digits
	}
	if e164 == "" && cc != "" && national != "" {
		e164 = "+" + cc + national
	}
	if e164 != "" && !strings.HasPrefix(e164, "+") {
		e164 = "+" + nonDigits.ReplaceAllString(e164, "")
	}
	if e164 == "" {
		return nil, fmt.Errorf("phone is required")
	}
	payload["workspace_id"] = workspaceID
	payload["request_id"] = firstNonEmpty(textField(payload, "request_id"), newRequestID("wa-req"))
	payload["job_id"] = firstNonEmpty(textField(payload, "job_id"), newRequestID(workflowJobPrefix(workflowPath)))
	payload["country_region"] = iso
	payload["country_code"] = cc
	payload["country_calling_code"] = cc
	payload["country_iso2"] = iso
	payload["region"] = iso
	payload["phone"] = map[string]any{"e164_number": e164, "country_calling_code": cc, "national_number": national, "country_iso2": iso}
	return json.Marshal(payload)
}

func resolveCountry(input string) struct {
	iso string
	cc  string
} {
	key := strings.ToUpper(strings.TrimSpace(input))
	if value, ok := countryByInput[key]; ok {
		return value
	}
	key = strings.TrimPrefix(key, "+")
	if value, ok := countryByInput[key]; ok {
		return value
	}
	if len(key) == 2 {
		return struct {
			iso string
			cc  string
		}{iso: key}
	}
	return struct {
		iso string
		cc  string
	}{iso: "US", cc: "1"}
}

func inferCountryFromE164Digits(digits string) struct {
	iso string
	cc  string
} {
	best := struct {
		iso string
		cc  string
	}{}
	for _, country := range countryByInput {
		if country.cc == "" || !strings.HasPrefix(digits, country.cc) {
			continue
		}
		if len(country.cc) > len(best.cc) {
			best = country
		}
	}
	return best
}

func textField(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return strings.TrimSpace(typed.String())
	case float64:
		return fmt.Sprintf("%.0f", typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func objectField(data map[string]any, key string) map[string]any {
	if data == nil {
		return map[string]any{}
	}
	if value, ok := data[key].(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func proxyRuntimeURL(baseURL string, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid PROXY_RUNTIME_API_BASE_URL")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func proxyURLFromEndpoint(baseURL string, endpoint map[string]any) (string, error) {
	port := int(numberField(endpoint, "port"))
	if port <= 0 {
		return "", fmt.Errorf("dynamic proxy lease has no egress port")
	}
	protocol := strings.ToUpper(textField(endpoint, "protocol"))
	scheme := "http"
	if strings.Contains(protocol, "SOCKS5") {
		scheme = "socks5"
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	host := textField(endpoint, "host")
	if isLocalProxyHost(host) {
		host = base.Hostname()
	}
	if host == "" {
		return "", fmt.Errorf("dynamic proxy lease has no egress host")
	}
	return (&url.URL{Scheme: scheme, Host: net.JoinHostPort(host, fmt.Sprintf("%d", port))}).String(), nil
}

func isLocalProxyHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	return host == "" || host == "0.0.0.0" || host == "127.0.0.1" || host == "localhost" || host == "::" || host == "::1"
}

func numberField(data map[string]any, key string) int64 {
	value, ok := data[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case json.Number:
		n, _ := typed.Int64()
		return n
	case float64:
		return int64(typed)
	case string:
		var n int64
		_, _ = fmt.Sscan(typed, &n)
		return n
	default:
		return 0
	}
}

func newRequestID(prefix string) string {
	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), hex.EncodeToString(random[:]))
}

func workflowJobPrefix(path string) string {
	if strings.Contains(path, "register") {
		return "wa-register"
	}
	return "wa-probe"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProtoJSON(w http.ResponseWriter, status int, value proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(value)
	if err != nil {
		_, _ = w.Write([]byte("{}"))
		return
	}
	_, _ = w.Write(data)
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func copyHeader(dst, src http.Header, key string) {
	if value := src.Get(key); value != "" {
		dst.Set(key, value)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func noCacheFileServer(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.NotFound(w, r)
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
