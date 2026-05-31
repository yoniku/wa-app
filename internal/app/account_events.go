package app

import (
	"context"
	"log"
	"time"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

func (s *Server) publishWAAccountUpserted(ctx context.Context, account *waappv1.WAAccount) {
	if s == nil || s.accountPublisher == nil || account == nil || account.GetAccount() == nil {
		return
	}
	publishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	_, err := s.accountPublisher.PublishUpserted(publishCtx, account.GetAccount())
	cancel()
	if err != nil && ctx.Err() == nil {
		log.Printf("publish WA account event failed account=%s: %v", waAccountID(account), sanitizeEventPublishError(err))
	}
}
