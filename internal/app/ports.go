package app

import (
	"context"
	"time"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

type Store interface {
	Close()
	SaveAppArtifact(context.Context, *waappv1.AppArtifact, string) error
	GetAppArtifact(context.Context, string, string) (*waappv1.AppArtifact, error)
	SaveProtocolProfile(context.Context, *waappv1.ProtocolProfile, string) error
	GetProtocolProfile(context.Context, string, string) (*waappv1.ProtocolProfile, error)

	SaveWAAccount(context.Context, *waappv1.WAAccount) error
	GetWAAccount(context.Context, string, string) (*waappv1.WAAccount, error)
	FindWAAccountByPhone(context.Context, string, string) (*waappv1.WAAccount, error)
	ListWAAccounts(context.Context, string, string, int) ([]*waappv1.WAAccount, string, error)
	SaveClientProfile(context.Context, *waappv1.ClientProfile, string) error
	GetClientProfile(context.Context, string, string) (*waappv1.ClientProfile, error)

	SaveAccountProbe(context.Context, *waappv1.AccountProbe, string) error
	SaveVerificationRequest(context.Context, *waappv1.VerificationCodeRequestRecord, string) error
	GetVerificationRequest(context.Context, string, string) (*waappv1.VerificationCodeRequestRecord, error)
	SaveRegistration(context.Context, *waappv1.RegistrationRecord, string) error
	GetRegistration(context.Context, string, string) (*waappv1.RegistrationRecord, error)
	SaveLoginState(context.Context, *waappv1.LoginState, string, string) error
	GetLoginState(context.Context, string, string) (*waappv1.LoginState, error)
	GetActiveLoginState(context.Context, string, string, string) (*waappv1.LoginState, error)
	ListActiveLoginStates(context.Context) ([]LoginStateRecord, error)
	GetLoginStateByRegistration(context.Context, string, string) (*waappv1.LoginState, error)
	GetLoginStateByRegisteredIdentity(context.Context, string, string) (*waappv1.LoginState, error)

	SaveMessageSession(context.Context, *waappv1.MessageSession, string) error
	GetMessageSession(context.Context, string, string) (*waappv1.MessageSession, error)
	SaveInboundMessages(context.Context, string, []*waappv1.InboundMessage) error
	GetInboundMessage(context.Context, string, string) (*waappv1.InboundMessage, error)
	SaveDecryptedMessage(context.Context, *waappv1.DecryptedMessage, string) error
	GetDecryptedMessage(context.Context, string, string) (*waappv1.DecryptedMessage, error)
	SaveCandidates(context.Context, string, []*waappv1.ExtractedCandidate) error
}

type RuntimeState interface {
	Close() error
	ClaimRequest(context.Context, string, time.Duration) (bool, error)
	SaveTransientState(context.Context, string, []byte, time.Duration) error
	GetTransientState(context.Context, string) ([]byte, error)
	DeleteTransientState(context.Context, string) error
	OpenSessionLease(context.Context, string, time.Duration) error
	CloseSessionLease(context.Context, string) error
}

type LoginStateRecord struct {
	WorkspaceID string
	LoginState  *waappv1.LoginState
}

type ProtocolEngine interface {
	PrepareClientProfile(context.Context, EngineProfileInput) error
	ProbeAccount(context.Context, EngineRegistrationInput) EngineProbeResult
	RequestVerificationCode(context.Context, EngineRegistrationInput) EngineCodeResult
	SubmitVerificationCode(context.Context, EngineSubmitInput) EngineRegisterResult
	CheckLoginState(context.Context, EngineLoginCheckInput) EngineLoginCheckResult
	ReceiveMessageBatch(context.Context, EngineMessageInput) EngineMessageBatchResult
	DecryptMessage(context.Context, EngineDecryptInput) EngineDecryptResult
}

type EngineProfileInput struct {
	WorkspaceID       string
	WAAccountID       string
	ClientProfileID   string
	ProtocolProfileID string
	Phone             *waappv1.PhoneTarget
}

type EngineRegistrationInput struct {
	WorkspaceID       string
	WAAccountID       string
	ClientProfileID   string
	ProtocolProfileID string
	Phone             *waappv1.PhoneTarget
}

type EngineSubmitInput struct {
	EngineRegistrationInput
	VerificationRequestID string
	Code                  string
	CodeSecretRef         string
}

type EngineLoginCheckInput struct {
	WorkspaceID          string
	WAAccountID          string
	ClientProfileID      string
	RegisteredIdentityID string
	RemoteTimeout        time.Duration
}

type EngineMessageInput struct {
	WorkspaceID          string
	WAAccountID          string
	ClientProfileID      string
	RegisteredIdentityID string
	ProtocolProfileID    string
	MessageSessionID     string
	WaitTimeout          time.Duration
	MaxMessages          int
}

type EngineDecryptInput struct {
	WorkspaceID          string
	MessageID            string
	MessageSessionID     string
	ClientProfileID      string
	PayloadRef           string
	SessionCommitPolicy  waappv1.SessionCommitPolicy
	IncludePlaintextText bool
}

type EngineProbeResult struct {
	Status           waappv1.AccountProbeStatus
	AccountFlow      string
	RawStatus        string
	RawReason        string
	RegisteredKnown  bool
	Registered       bool
	Blocked          bool
	SMSWaitSeconds   int64
	CanSendSMS       bool
	SupportedMethods []waappv1.VerificationDeliveryMethod
	MethodStatuses   []VerificationMethodStatus
	Err              error
}

type VerificationMethodStatus struct {
	Method          waappv1.VerificationDeliveryMethod
	Available       bool
	CooldownSeconds int64
}

const (
	accountProbeFlowUnknown       = "unknown"
	accountProbeFlowProbeFailed   = "probe_failed"
	accountProbeFlowRegistered    = "registered"
	accountProbeFlowNotRegistered = "not_registered"
	accountProbeFlowBlocked       = "blocked"
	accountProbeFlowInvalidNumber = "invalid_number"
	accountProbeFlowRateLimited   = "rate_limited"
)

type EngineCodeResult struct {
	Status             waappv1.VerificationRequestStatus
	ExpectedCodeLength int32
	ExpiresAt          time.Time
	Err                error
}

type EngineRegisterResult struct {
	Status           waappv1.RegistrationStatus
	RegisteredID     string
	ServiceAccountID string
	ServiceLoginID   string
	CompletedAt      time.Time
	Err              error
}

type EngineLoginCheckResult struct {
	Status waappv1.LoginStateCheckStatus
	Err    error
}

type EngineMessageBatchResult struct {
	Messages []*waappv1.InboundMessage
	Err      error
}

type EngineDecryptResult struct {
	DecryptedMessage *waappv1.DecryptedMessage
	Candidates       []*waappv1.ExtractedCandidate
	Err              error
}
