package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"strconv"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth/model"
)

type OTP struct {
	repository   Repository
	notifier     Notifier
	now          func() time.Time
	random       io.Reader
	generateCode func() (string, error)
}

func NewOTP(repository Repository, notifier Notifier) *OTP {
	return &OTP{
		repository: repository,
		notifier:   notifier,
		now:        time.Now,
		random:     rand.Reader,
		generateCode: func() (string, error) {
			var digits [model.CodeLength]byte
			if _, err := rand.Read(digits[:]); err != nil {
				return "", model.ErrUnavailable
			}
			code := make([]byte, model.CodeLength)
			for i, digit := range digits {
				code[i] = '0' + digit%10
			}
			return string(code), nil
		},
	}
}

func (o *OTP) Request(ctx context.Context, command model.RequestCommand) (model.Challenge, error) {
	if o == nil || o.repository == nil || o.notifier == nil {
		return model.Challenge{}, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Challenge{}, err
	}
	challengeID, err := o.challengeID()
	if err != nil {
		return model.Challenge{}, err
	}
	code, err := o.generateCode()
	if err != nil || !model.ValidCode(code) {
		return model.Challenge{}, model.ErrUnavailable
	}
	now := o.now().UTC()
	record, err := o.repository.CreatePending(ctx, model.Record{
		ID:         challengeID,
		ShopCode:   normalized.ShopCode,
		Phone:      normalized.Phone,
		Email:      normalized.Email,
		CodeHash:   model.HashCode(challengeID, code),
		TTLSeconds: model.TTLSeconds,
		Status:     model.StatusPending,
		ExpiresAt:  now.Add(time.Duration(model.TTLSeconds) * time.Second),
		CreatedAt:  now,
	})
	if err != nil {
		return model.Challenge{}, err
	}
	deliveries, err := o.notifier.Dispatch(ctx, Dispatch{
		EventKey:    model.EventKey,
		DeliveryKey: model.DeliveryKey(record.ID),
		MerchantID:  record.MerchantID,
		ShopID:      record.ShopID,
		Phone:       record.Phone,
		Email:       record.Email,
		Variables: map[string]string{
			"code":       code,
			"ttlSeconds": strconv.Itoa(model.TTLSeconds),
		},
	})
	if err != nil || !model.Delivered(deliveries) {
		return model.Challenge{}, model.ErrDeliveryFailed
	}
	return model.Challenge{ID: record.ID, TTLSeconds: record.TTLSeconds, ExpiresAt: record.ExpiresAt}, nil
}

func (o *OTP) Verify(ctx context.Context, command model.VerifyCommand) (model.Challenge, error) {
	if o == nil || o.repository == nil {
		return model.Challenge{}, model.ErrUnavailable
	}
	normalized, err := command.Normalize()
	if err != nil {
		return model.Challenge{}, err
	}
	if err := o.repository.Consume(ctx, normalized, model.HashCode(normalized.ChallengeID, normalized.Code), o.now().UTC()); err != nil {
		return model.Challenge{}, err
	}
	return model.Challenge{ID: normalized.ChallengeID, TTLSeconds: model.TTLSeconds}, nil
}

func (o *OTP) challengeID() (string, error) {
	reader := o.random
	if reader == nil {
		reader = rand.Reader
	}
	var raw [32]byte
	if _, err := io.ReadFull(reader, raw[:]); err != nil {
		return "", model.ErrUnavailable
	}
	return hex.EncodeToString(raw[:]), nil
}
