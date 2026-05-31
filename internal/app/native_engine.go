package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NativeEngineConfig struct {
	StateRoot          string
	ExistURL           string
	CodeURL            string
	RegisterURL        string
	ServerPublicKeyHex string
	RegistrationToken  string
	UserAgent          string
	AppVersion         string
	ProxyURL           string
	HTTPTimeout        time.Duration
	InsecureTLS        bool
	ChatdHost          string
	ChatdPort          int
	ChatdTLS           bool
	ChatdRoutingInfo   string
	ChatdTimeout       time.Duration
	ChatdMaxFrameBytes int
}

type NativeEngine struct {
	cfg   NativeEngineConfig
	http  *nativeHTTPClient
	clock Clock
	ids   IDGenerator
}

func NewNativeEngine(cfg NativeEngineConfig, clock Clock, ids IDGenerator) (*NativeEngine, error) {
	if cfg.StateRoot == "" {
		return nil, fmt.Errorf("WA_APP_STATE_DIR is required")
	}
	if cfg.AppVersion == "" {
		cfg.AppVersion = "2.26.21.73"
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = nativeUserAgent(cfg.AppVersion)
	}
	cfg.ProxyURL = strings.TrimSpace(cfg.ProxyURL)
	if cfg.CodeURL == "" {
		cfg.CodeURL = "https://y9yrsygcg6.execute-api.us-east-1.amazonaws.com/s/s?_=/v2/code&"
	}
	if cfg.ExistURL == "" {
		cfg.ExistURL = "https://y9yrsygcg6.execute-api.us-east-1.amazonaws.com/s/s?_=/v2/exist&"
	}
	if cfg.RegisterURL == "" {
		cfg.RegisterURL = "https://y9yrsygcg6.execute-api.us-east-1.amazonaws.com/s/s?_=/v2/register&"
	}
	if cfg.ChatdHost == "" {
		cfg.ChatdHost = defaultChatdHost
	}
	if cfg.ChatdPort == 0 {
		cfg.ChatdPort = defaultChatdPort
	}
	if cfg.ChatdTimeout <= 0 {
		cfg.ChatdTimeout = 15 * time.Second
	}
	if cfg.ChatdMaxFrameBytes <= 0 {
		cfg.ChatdMaxFrameBytes = defaultChatdMaxFrame
	}
	if clock == nil {
		clock = SystemClock{}
	}
	if ids == nil {
		ids = RandomIDGenerator{}
	}
	hc, err := newNativeHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return &NativeEngine{cfg: cfg, http: hc, clock: clock, ids: ids}, nil
}

func (e *NativeEngine) WithProxyURL(proxyURL string) (*NativeEngine, error) {
	cfg := e.cfg
	cfg.ProxyURL = strings.TrimSpace(proxyURL)
	hc, err := newNativeHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return &NativeEngine{cfg: cfg, http: hc, clock: e.clock, ids: e.ids}, nil
}

func (e *NativeEngine) PrepareClientProfile(ctx context.Context, input EngineProfileInput) error {
	_ = ctx
	state, err := newNativeState(input.Phone, e.cfg.AppVersion)
	if err != nil {
		return err
	}
	return saveNativeState(e.profileDir(input.ClientProfileID), state)
}

func (e *NativeEngine) ProbeAccount(ctx context.Context, input EngineRegistrationInput) EngineProbeResult {
	state, err := e.loadState(input.ClientProfileID)
	if err != nil {
		return EngineProbeResult{Status: waappv1.AccountProbeStatus_ACCOUNT_PROBE_STATUS_REJECTED, Err: err}
	}
	return e.probeAccountWithState(ctx, input, state)
}

func (e *NativeEngine) probeAccountWithState(ctx context.Context, input EngineRegistrationInput, state nativeState) EngineProbeResult {
	params := map[string]string{"cc": phoneCC(input.Phone), "in": phoneNational(input.Phone), "lg": "en", "lc": "US"}
	if token := firstNonEmpty(e.cfg.RegistrationToken, state.LastCodeParams["token"]); token != "" {
		params["token"] = token
	}
	plain := renderNativePlain(params, nil)
	client, err := e.httpForProxy()
	if err != nil {
		return EngineProbeResult{Status: waappv1.AccountProbeStatus_ACCOUNT_PROBE_STATUS_REJECTED, Err: err}
	}
	data, _, err := client.postWASafe(ctx, e.cfg.ExistURL, plain, state.UserAgent)
	status := waappv1.AccountProbeStatus_ACCOUNT_PROBE_STATUS_UNKNOWN
	if err != nil {
		return EngineProbeResult{Status: waappv1.AccountProbeStatus_ACCOUNT_PROBE_STATUS_REJECTED, Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_REJECTED, "account probe failed", true)}
	}
	s := responseStatus(data)
	if s == "ok" || s == "sent" || s == "valid" || s == "exists" {
		status = waappv1.AccountProbeStatus_ACCOUNT_PROBE_STATUS_REACHABLE
	} else if s == "blocked" || s == "no_routes" || s == "rejected" {
		status = waappv1.AccountProbeStatus_ACCOUNT_PROBE_STATUS_UNREACHABLE
	}
	return EngineProbeResult{Status: status, SupportedMethods: []waappv1.VerificationDeliveryMethod{waappv1.VerificationDeliveryMethod_VERIFICATION_DELIVERY_METHOD_SMS}}
}

func (e *NativeEngine) RequestVerificationCode(ctx context.Context, input EngineRegistrationInput) EngineCodeResult {
	state, err := e.loadState(input.ClientProfileID)
	if err != nil {
		return EngineCodeResult{Status: waappv1.VerificationRequestStatus_VERIFICATION_REQUEST_STATUS_REJECTED, Err: err}
	}
	result, updated := e.requestVerificationCodeWithState(ctx, input, state)
	_ = saveNativeState(e.profileDir(input.ClientProfileID), updated)
	return result
}

func (e *NativeEngine) requestVerificationCodeWithState(ctx context.Context, input EngineRegistrationInput, state nativeState) (EngineCodeResult, nativeState) {
	params, rawKeys := e.codeParams(input.Phone, state)
	plain := renderNativePlain(params, rawKeys)
	client, err := e.httpForProxy()
	if err != nil {
		return EngineCodeResult{Status: waappv1.VerificationRequestStatus_VERIFICATION_REQUEST_STATUS_REJECTED, Err: err}, state
	}
	data, enc, err := client.postWASafe(ctx, e.cfg.CodeURL, plain, state.UserAgent)
	state.LastCodeParams = params
	state.LastCodeResult = sanitizeResponse(data)
	if enc != "" {
		state.LastCodeResult["enc_sha256"] = encHash(enc)
	}
	if err != nil {
		return EngineCodeResult{Status: waappv1.VerificationRequestStatus_VERIFICATION_REQUEST_STATUS_REJECTED, Err: classifyHTTPError(data, err)}, state
	}
	status := waappv1.VerificationRequestStatus_VERIFICATION_REQUEST_STATUS_WAITING
	s := responseStatus(data)
	if s == "sent" || s == "ok" {
		status = waappv1.VerificationRequestStatus_VERIFICATION_REQUEST_STATUS_SENT
	} else if s != "" && s != "too_recent" {
		status = waappv1.VerificationRequestStatus_VERIFICATION_REQUEST_STATUS_REJECTED
	}
	return EngineCodeResult{Status: status, ExpectedCodeLength: int32(jsonNumber(data["length"])), ExpiresAt: e.clock.Now().Add(10 * time.Minute)}, state
}

func (e *NativeEngine) SubmitVerificationCode(ctx context.Context, input EngineSubmitInput) EngineRegisterResult {
	if strings.TrimSpace(input.Code) == "" {
		return EngineRegisterResult{Status: waappv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED, Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_VALIDATION_FAILED, "verification code is required", false)}
	}
	state, err := e.loadState(input.ClientProfileID)
	if err != nil {
		return EngineRegisterResult{Status: waappv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED, Err: err}
	}
	params, rawKeys := e.registerParams(input.Phone, input.Code, state)
	plain := renderNativePlain(params, rawKeys)
	client, err := e.httpForProxy()
	if err != nil {
		return EngineRegisterResult{Status: waappv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED, Err: err}
	}
	data, enc, err := client.postWASafe(ctx, e.cfg.RegisterURL, plain, state.UserAgent)
	state.LastRegister = sanitizeResponse(data)
	if enc != "" {
		state.LastRegister["enc_sha256"] = encHash(enc)
	}
	if err != nil {
		_ = saveNativeState(e.profileDir(input.ClientProfileID), state)
		return EngineRegisterResult{Status: waappv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED, Err: classifyHTTPError(data, err)}
	}
	if status := responseStatus(data); status != "ok" && status != "registered" {
		_ = saveNativeState(e.profileDir(input.ClientProfileID), state)
		return EngineRegisterResult{Status: waappv1.RegistrationStatus_REGISTRATION_STATUS_REJECTED, Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_REJECTED, "registration was rejected", false)}
	}
	login := firstNonEmpty(jsonString(data["login"]), jsonString(data["jid"]), jsonString(data["registration_jid"]), state.CC+state.Phone)
	lid := firstNonEmpty(jsonString(data["lid"]), login)
	if login != "" {
		state.RegistrationJID = normalizeJID(login)
	}
	_ = saveNativeState(e.profileDir(input.ClientProfileID), state)
	completedAt := e.clock.Now()
	return EngineRegisterResult{Status: waappv1.RegistrationStatus_REGISTRATION_STATUS_REGISTERED, RegisteredID: "waid_" + stableID(login), ServiceAccountID: lid, ServiceLoginID: login, CompletedAt: completedAt}
}

func (e *NativeEngine) CheckLoginState(ctx context.Context, input EngineLoginCheckInput) EngineLoginCheckResult {
	state, err := e.loadState(input.ClientProfileID)
	if err != nil {
		return EngineLoginCheckResult{Status: waappv1.LoginStateCheckStatus_LOGIN_STATE_CHECK_STATUS_INVALID, Err: err}
	}
	proxyURL, err := e.proxyURL()
	if err != nil {
		return EngineLoginCheckResult{Status: waappv1.LoginStateCheckStatus_LOGIN_STATE_CHECK_STATUS_UNSPECIFIED, Err: err}
	}
	timeout := e.cfg.ChatdTimeout
	if input.RemoteTimeout > 0 {
		timeout = input.RemoteTimeout
	}
	client := newChatdClient(chatdClientConfig{Host: e.cfg.ChatdHost, Port: e.cfg.ChatdPort, TLS: e.cfg.ChatdTLS, RoutingInfo: e.cfg.ChatdRoutingInfo, ProxyURL: proxyURL, InsecureTLS: e.cfg.InsecureTLS, Timeout: timeout, MaxFrameBytes: e.cfg.ChatdMaxFrameBytes})
	if err := client.checkLoginState(ctx, state, input, e.cfg.AppVersion); err != nil {
		status := loginCheckStatusForError(err)
		return EngineLoginCheckResult{Status: status, Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_REJECTED, "login state remote check failed", status == waappv1.LoginStateCheckStatus_LOGIN_STATE_CHECK_STATUS_UNREACHABLE)}
	}
	return EngineLoginCheckResult{Status: waappv1.LoginStateCheckStatus_LOGIN_STATE_CHECK_STATUS_ACTIVE}
}

func loginCheckStatusForError(err error) waappv1.LoginStateCheckStatus {
	if err == nil {
		return waappv1.LoginStateCheckStatus_LOGIN_STATE_CHECK_STATUS_ACTIVE
	}
	lower := strings.ToLower(err.Error())
	for _, marker := range []string{"timeout", "deadline", "proxy", "dial", "connect", "network", "tls", "no such host", "connection refused", "temporary"} {
		if strings.Contains(lower, marker) {
			return waappv1.LoginStateCheckStatus_LOGIN_STATE_CHECK_STATUS_UNREACHABLE
		}
	}
	return waappv1.LoginStateCheckStatus_LOGIN_STATE_CHECK_STATUS_INVALID
}

func (e *NativeEngine) ReceiveMessageBatch(ctx context.Context, input EngineMessageInput) EngineMessageBatchResult {
	state, err := e.loadState(input.ClientProfileID)
	if err != nil {
		return EngineMessageBatchResult{Err: err}
	}
	state.ensureMaps()
	if state.ChatStatic.Private == "" || state.ChatStatic.Public == "" {
		state.ChatStatic = ensureChatStatic(state.ChatStatic)
		_ = saveNativeState(e.profileDir(input.ClientProfileID), state)
	}
	proxyURL, err := e.proxyURL()
	if err != nil {
		return EngineMessageBatchResult{Err: err}
	}
	client := newChatdClient(chatdClientConfig{Host: e.cfg.ChatdHost, Port: e.cfg.ChatdPort, TLS: e.cfg.ChatdTLS, RoutingInfo: e.cfg.ChatdRoutingInfo, ProxyURL: proxyURL, InsecureTLS: e.cfg.InsecureTLS, Timeout: e.cfg.ChatdTimeout, MaxFrameBytes: e.cfg.ChatdMaxFrameBytes})
	messages, payloads, err := client.receiveBatch(ctx, state, input, e.cfg.AppVersion, e.ids, e.clock.Now())
	if err != nil {
		return EngineMessageBatchResult{Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_REJECTED, "native chatd receive failed", true)}
	}
	for _, payload := range payloads {
		ref := payloadRefForEnc(input.MessageSessionID, payload.Payload)
		state.MessagePayloads[ref] = nativeMessagePayload{Sender: payload.Sender, EncType: payload.EncType, Path: payload.Path, Payload: b64u(payload.Payload)}
	}
	_ = saveNativeState(e.profileDir(input.ClientProfileID), state)
	return EngineMessageBatchResult{Messages: messages}
}

func (e *NativeEngine) DecryptMessage(ctx context.Context, input EngineDecryptInput) EngineDecryptResult {
	_ = ctx
	if strings.HasPrefix(input.PayloadRef, "plaintext:") {
		plain := strings.TrimPrefix(input.PayloadRef, "plaintext:")
		decryptedID := e.ids.NewID("wadec_")
		text := &waappv1.SensitiveText{RedactedValue: redacted(plain), SecretRef: "plaintext:" + decryptedID}
		if input.IncludePlaintextText {
			text.Value = plain
		}
		msg := &waappv1.DecryptedMessage{DecryptedMessageId: decryptedID, MessageId: input.MessageID, Status: waappv1.DecryptionStatus_DECRYPTION_STATUS_DECRYPTED, PlaintextRef: "inline:" + decryptedID, PlaintextText: text, DecryptedAt: timestamppb.New(e.clock.Now())}
		return EngineDecryptResult{DecryptedMessage: msg, Candidates: extractCandidates(input.MessageID, decryptedID, plain, input.IncludePlaintextText, e.clock.Now(), e.ids)}
	}
	if strings.HasPrefix(input.PayloadRef, "native-enc:") {
		state, err := e.loadState(input.ClientProfileID)
		if err != nil {
			return EngineDecryptResult{Err: err}
		}
		payload, ok := state.MessagePayloads[input.PayloadRef]
		if !ok {
			return EngineDecryptResult{Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_MESSAGE_NOT_FOUND, "encrypted message payload ref not found", false)}
		}
		commit := input.SessionCommitPolicy == waappv1.SessionCommitPolicy_SESSION_COMMIT_POLICY_COMMIT_LEARNED_STATE
		output, err := decryptNativeSignalPayload(&state, payload, commit)
		if err != nil {
			return EngineDecryptResult{Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_DECRYPTION_FAILED, "native Signal message decryption failed", true)}
		}
		if commit {
			_ = saveNativeState(e.profileDir(input.ClientProfileID), state)
		}
		decryptedID := e.ids.NewID("wadec_")
		plain := string(output.plaintext)
		text := &waappv1.SensitiveText{RedactedValue: redacted(plain), SecretRef: "native-plain:" + decryptedID}
		if input.IncludePlaintextText {
			text.Value = plain
		}
		msg := &waappv1.DecryptedMessage{DecryptedMessageId: decryptedID, MessageId: input.MessageID, Status: waappv1.DecryptionStatus_DECRYPTION_STATUS_DECRYPTED, PlaintextRef: "native-plain:" + decryptedID, PlaintextText: text, DecryptedAt: timestamppb.New(e.clock.Now())}
		return EngineDecryptResult{DecryptedMessage: msg, Candidates: extractCandidates(input.MessageID, decryptedID, plain, input.IncludePlaintextText, e.clock.Now(), e.ids)}
	}
	return EngineDecryptResult{Err: NewError(waappv1.WaErrorCode_WA_ERROR_CODE_UNSUPPORTED_OPERATION, "payload ref scheme is not supported by native decryptor", false)}
}

func (e *NativeEngine) codeParams(phone *waappv1.PhoneTarget, state nativeState) (map[string]string, map[string]struct{}) {
	params := map[string]string{
		"cc":                phoneCC(phone),
		"in":                phoneNational(phone),
		"method":            "sms",
		"lg":                "en",
		"lc":                "US",
		"fdid":              state.Profile.FDID,
		"expid":             state.Profile.ExpID,
		"access_session_id": state.Profile.AccessSessionID,
		"id":                state.Profile.ID,
		"backup_token":      state.Profile.BackupToken,
		"authkey":           state.AuthKey,
		"e_ident":           state.KeyBundle.IdentityPublic,
		"e_keytype":         state.KeyBundle.KeyType,
		"e_regid":           state.KeyBundle.RegID,
		"e_skey_id":         state.KeyBundle.SignedKeyID,
		"e_skey_val":        state.KeyBundle.SignedKeyValue,
		"e_skey_sig":        state.KeyBundle.SignedKeySig,
	}
	if e.cfg.RegistrationToken != "" {
		params["token"] = e.cfg.RegistrationToken
	}
	raw := map[string]struct{}{"id": {}, "backup_token": {}}
	for key, value := range state.Profile.AdditionalMapFields {
		params[key] = pctBytes([]byte(value))
		raw[key] = struct{}{}
	}
	return params, raw
}

func (e *NativeEngine) registerParams(phone *waappv1.PhoneTarget, code string, state nativeState) (map[string]string, map[string]struct{}) {
	params := state.codeParams()
	if len(params) == 0 {
		params, _ = e.codeParams(phone, state)
	}
	params["cc"] = phoneCC(phone)
	params["in"] = phoneNational(phone)
	params["code"] = code
	params["method"] = "sms"
	if token := firstNonEmpty(params["token"], e.cfg.RegistrationToken); token != "" {
		params["token"] = token
	}
	return params, map[string]struct{}{"id": {}, "backup_token": {}}
}

func (e *NativeEngine) loadState(clientProfileID string) (nativeState, error) {
	state, err := loadNativeState(e.profileDir(clientProfileID))
	if err != nil {
		return nativeState{}, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_PROFILE_NOT_FOUND, "native client profile state not found", false)
	}
	return state, nil
}

func (e *NativeEngine) newState(phone *waappv1.PhoneTarget) (nativeState, error) {
	return newNativeState(phone, e.cfg.AppVersion)
}

func (e *NativeEngine) saveState(clientProfileID string, state nativeState) error {
	return saveNativeState(e.profileDir(clientProfileID), state)
}

func (e *NativeEngine) profileDir(clientProfileID string) string {
	return filepath.Join(e.cfg.StateRoot, clientProfileID)
}

func sanitizeResponse(data map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range data {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "auth") || strings.Contains(lower, "code") || strings.Contains(lower, "sig") {
			out[key] = "<redacted>"
			continue
		}
		out[key] = value
	}
	return out
}

func classifyHTTPError(data map[string]any, err error) error {
	status := responseStatus(data)
	switch status {
	case "no_routes":
		return NewError(waappv1.WaErrorCode_WA_ERROR_CODE_ROUTE_UNAVAILABLE, "verification route is unavailable", false)
	case "too_recent":
		return NewError(waappv1.WaErrorCode_WA_ERROR_CODE_RATE_LIMITED, "verification request is too recent", true)
	case "blocked", "rejected":
		return NewError(waappv1.WaErrorCode_WA_ERROR_CODE_REJECTED, "request was rejected", false)
	}
	return NewError(waappv1.WaErrorCode_WA_ERROR_CODE_REJECTED, err.Error(), true)
}

func jsonString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func jsonNumber(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func extractCandidates(messageID string, decryptedID string, text string, includeValue bool, now time.Time, ids IDGenerator) []*waappv1.ExtractedCandidate {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	patterns := []struct {
		kind waappv1.CandidateKind
		re   *regexp.Regexp
	}{
		{waappv1.CandidateKind_CANDIDATE_KIND_FLAG, regexp.MustCompile(`(?i)(flag|ctf)\{[^\s}]{1,120}\}`)},
		{waappv1.CandidateKind_CANDIDATE_KIND_OTP, regexp.MustCompile(`\b\d{4,8}\b`)},
	}
	out := []*waappv1.ExtractedCandidate{}
	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		for _, match := range pattern.re.FindAllString(text, -1) {
			key := pattern.kind.String() + ":" + match
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			candidateID := ids.NewID("wacand_")
			sensitive := &waappv1.SensitiveText{RedactedValue: redacted(match), SecretRef: "candidate:" + candidateID}
			if includeValue {
				sensitive.Value = match
			}
			out = append(out, &waappv1.ExtractedCandidate{CandidateId: candidateID, MessageId: messageID, DecryptedMessageId: decryptedID, Kind: pattern.kind, Text: sensitive, Confidence: 0.9, ExtractedAt: timestamppb.New(now)})
		}
	}
	return out
}
