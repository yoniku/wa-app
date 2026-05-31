package app

import (
	"context"

	wav1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/wa/v1"
	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

func (s *Server) DecryptMessage(ctx context.Context, req *waappv1.DecryptMessageRequest) (*waappv1.DecryptMessageResponse, error) {
	return s.decryptMessage(ctx, req, s.runner, wav1.WaOtpSource_WA_OTP_SOURCE_MANUAL_EXTRACTION)
}

func (s *Server) decryptMessage(ctx context.Context, req *waappv1.DecryptMessageRequest, runner ProtocolEngine, otpSource wav1.WaOtpSource) (*waappv1.DecryptMessageResponse, error) {
	if err := validateContext(req.GetContext()); err != nil {
		return &waappv1.DecryptMessageResponse{Error: ToProtoError(err)}, nil
	}
	workspaceID := req.GetContext().GetWorkspaceId()
	msg, err := s.store.GetInboundMessage(ctx, workspaceID, req.GetMessageId())
	if err != nil {
		return &waappv1.DecryptMessageResponse{Error: ToProtoError(err)}, nil
	}
	session, err := s.store.GetMessageSession(ctx, workspaceID, msg.GetMessageSessionId())
	if err != nil {
		return &waappv1.DecryptMessageResponse{Error: ToProtoError(err)}, nil
	}
	if runner == nil {
		runner = s.runner
	}
	result := runner.DecryptMessage(ctx, EngineDecryptInput{WorkspaceID: workspaceID, MessageID: msg.GetMessageId(), MessageSessionID: msg.GetMessageSessionId(), ClientProfileID: session.GetClientProfileId(), PayloadRef: msg.GetPayloadRef(), SessionCommitPolicy: req.GetSessionCommitPolicy(), IncludePlaintextText: req.GetIncludeSensitivePlaintext()})
	if result.Err != nil {
		return &waappv1.DecryptMessageResponse{Error: ToProtoError(result.Err)}, nil
	}
	if err := s.store.SaveDecryptedMessage(ctx, result.DecryptedMessage, workspaceID); err != nil {
		return &waappv1.DecryptMessageResponse{Error: ToProtoError(err)}, nil
	}
	if len(result.Candidates) > 0 {
		_ = s.store.SaveCandidates(ctx, workspaceID, result.Candidates)
		s.publishOTPCandidates(context.WithoutCancel(ctx), req.GetContext(), workspaceID, msg, session, result.Candidates, otpSource)
	}
	msg.EncryptionState = waappv1.MessageEncryptionState_MESSAGE_ENCRYPTION_STATE_DECRYPTED
	_ = s.store.SaveInboundMessages(ctx, workspaceID, []*waappv1.InboundMessage{msg})
	return &waappv1.DecryptMessageResponse{DecryptedMessage: result.DecryptedMessage}, nil
}

func (s *Server) ExtractCandidates(ctx context.Context, req *waappv1.ExtractCandidatesRequest) (*waappv1.ExtractCandidatesResponse, error) {
	if err := validateContext(req.GetContext()); err != nil {
		return &waappv1.ExtractCandidatesResponse{Error: ToProtoError(err)}, nil
	}
	workspaceID := req.GetContext().GetWorkspaceId()
	messageID := req.GetMessageId()
	if messageID == "" {
		decrypted, err := s.store.GetDecryptedMessage(ctx, workspaceID, req.GetDecryptedMessageId())
		if err != nil {
			return &waappv1.ExtractCandidatesResponse{Error: ToProtoError(err)}, nil
		}
		messageID = decrypted.GetMessageId()
	}
	msg, err := s.store.GetInboundMessage(ctx, workspaceID, messageID)
	if err != nil {
		return &waappv1.ExtractCandidatesResponse{Error: ToProtoError(err)}, nil
	}
	session, err := s.store.GetMessageSession(ctx, workspaceID, msg.GetMessageSessionId())
	if err != nil {
		return &waappv1.ExtractCandidatesResponse{Error: ToProtoError(err)}, nil
	}
	result := s.runner.DecryptMessage(ctx, EngineDecryptInput{WorkspaceID: workspaceID, MessageID: msg.GetMessageId(), MessageSessionID: msg.GetMessageSessionId(), ClientProfileID: session.GetClientProfileId(), PayloadRef: msg.GetPayloadRef(), SessionCommitPolicy: waappv1.SessionCommitPolicy_SESSION_COMMIT_POLICY_TRANSIENT, IncludePlaintextText: req.GetIncludeSensitiveValues()})
	if result.Err != nil {
		return &waappv1.ExtractCandidatesResponse{Error: ToProtoError(result.Err)}, nil
	}
	candidates := filterCandidates(result.Candidates, req.GetCandidateKinds())
	if err := s.store.SaveCandidates(ctx, workspaceID, candidates); err != nil {
		return &waappv1.ExtractCandidatesResponse{Error: ToProtoError(err)}, nil
	}
	s.publishOTPCandidates(context.WithoutCancel(ctx), req.GetContext(), workspaceID, msg, session, candidates, wav1.WaOtpSource_WA_OTP_SOURCE_MANUAL_EXTRACTION)
	return &waappv1.ExtractCandidatesResponse{Candidates: candidates}, nil
}

func filterCandidates(candidates []*waappv1.ExtractedCandidate, kinds []waappv1.CandidateKind) []*waappv1.ExtractedCandidate {
	if len(kinds) == 0 {
		return candidates
	}
	allowed := map[waappv1.CandidateKind]struct{}{}
	for _, kind := range kinds {
		allowed[kind] = struct{}{}
	}
	out := []*waappv1.ExtractedCandidate{}
	for _, candidate := range candidates {
		if _, ok := allowed[candidate.GetKind()]; ok {
			out = append(out, candidate)
		}
	}
	return out
}
