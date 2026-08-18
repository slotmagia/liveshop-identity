package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth/model"
)

type memoryRepository struct {
	mu          sync.Mutex
	shops       map[string]struct{ merchantID, shopID int64 }
	records     map[string]model.Record
	createCalls int
}

func (m *memoryRepository) CreatePending(_ context.Context, record model.Record) (model.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	shop, ok := m.shops[record.ShopCode]
	if !ok {
		return model.Record{}, model.ErrInvalid
	}
	for id, existing := range m.records {
		if existing.ShopID == shop.shopID && existing.Phone == record.Phone && existing.Email == record.Email && existing.Status == model.StatusPending {
			existing.Status = model.StatusExpired
			m.records[id] = existing
		}
	}
	record.MerchantID = shop.merchantID
	record.ShopID = shop.shopID
	record.Status = model.StatusPending
	m.records[record.ID] = record
	m.createCalls++
	return record, nil
}

func (m *memoryRepository) Consume(_ context.Context, command model.VerifyCommand, codeHash string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[command.ChallengeID]
	if !ok {
		return model.ErrNotFound
	}
	if record.ShopCode != command.ShopCode {
		return model.ErrInvalid
	}
	if record.Status != model.StatusPending || !record.ExpiresAt.After(now) {
		record.Status = model.StatusExpired
		m.records[record.ID] = record
		return model.ErrExpired
	}
	if record.CodeHash != codeHash {
		record.AttemptCount++
		if record.AttemptCount >= model.MaxAttempts {
			record.Status = model.StatusExpired
			m.records[record.ID] = record
			return model.ErrExpired
		}
		m.records[record.ID] = record
		return model.ErrInvalid
	}
	record.Status = model.StatusConsumed
	m.records[record.ID] = record
	return nil
}

func (m *memoryRepository) Get(_ context.Context, id string) (model.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.records[id]
	if !ok {
		return model.Record{}, model.ErrNotFound
	}
	return record, nil
}

type recordingNotifier struct {
	mu         sync.Mutex
	calls      int
	deliveries []model.Delivery
	err        error
	last       Dispatch
	before     func()
}

func (n *recordingNotifier) Dispatch(_ context.Context, message Dispatch) ([]model.Delivery, error) {
	if n.before != nil {
		n.before()
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls++
	n.last = message
	if n.err != nil {
		return nil, n.err
	}
	return n.deliveries, nil
}

func newOTP(repo *memoryRepository, notifier *recordingNotifier) *OTP {
	otp := NewOTP(repo, notifier)
	otp.now = func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) }
	otp.generateCode = func() (string, error) { return "123456", nil }
	return otp
}

func TestRequestDispatchesAfterChallengeCommit(t *testing.T) {
	repo := &memoryRepository{
		shops:   map[string]struct{ merchantID, shopID int64 }{"local-shop": {2001, 3001}},
		records: map[string]model.Record{},
	}
	notifier := &recordingNotifier{deliveries: []model.Delivery{{Channel: "SMS", Status: model.StatusSent}}}
	notifier.before = func() {
		if repo.createCalls != 1 {
			t.Fatalf("dispatch ran before challenge commit: createCalls=%d", repo.createCalls)
		}
	}
	challenge, err := newOTP(repo, notifier).Request(context.Background(), model.RequestCommand{
		ShopCode: "local-shop", Phone: "13800000000",
	})
	if err != nil || challenge.ID == "" || challenge.TTLSeconds != model.TTLSeconds {
		t.Fatalf("challenge=%+v err=%v", challenge, err)
	}
	if notifier.calls != 1 || notifier.last.EventKey != model.EventKey || notifier.last.DeliveryKey != model.DeliveryKey(challenge.ID) {
		t.Fatalf("dispatch=%+v", notifier.last)
	}
	if notifier.last.Variables["code"] != "123456" || notifier.last.Variables["ttlSeconds"] != "300" {
		t.Fatalf("variables=%v", notifier.last.Variables)
	}
	stored, err := repo.Get(context.Background(), challenge.ID)
	if err != nil || stored.CodeHash == "123456" || stored.Status != model.StatusPending {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
}

func TestRequestKeepsChallengeWhenDeliveryFails(t *testing.T) {
	repo := &memoryRepository{
		shops:   map[string]struct{ merchantID, shopID int64 }{"local-shop": {2001, 3001}},
		records: map[string]model.Record{},
	}
	notifier := &recordingNotifier{deliveries: []model.Delivery{
		{Channel: "SMS", Status: "FAILED_PERMANENT"},
		{Channel: "EMAIL", Status: "FAILED_PERMANENT"},
	}}
	_, err := newOTP(repo, notifier).Request(context.Background(), model.RequestCommand{
		ShopCode: "local-shop", Email: "buyer@example.com",
	})
	if err != model.ErrDeliveryFailed {
		t.Fatalf("err=%v", err)
	}
	if len(repo.records) != 1 {
		t.Fatalf("challenge was deleted after send failure: %+v", repo.records)
	}
	for _, record := range repo.records {
		if record.Status != model.StatusPending {
			t.Fatalf("status=%s", record.Status)
		}
	}
}

func TestVerifyConsumesMatchingCode(t *testing.T) {
	repo := &memoryRepository{
		shops:   map[string]struct{ merchantID, shopID int64 }{"local-shop": {2001, 3001}},
		records: map[string]model.Record{},
	}
	otp := newOTP(repo, &recordingNotifier{deliveries: []model.Delivery{{Channel: "SMS", Status: model.StatusSent}}})
	challenge, err := otp.Request(context.Background(), model.RequestCommand{ShopCode: "local-shop", Phone: "13800000000"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := otp.Verify(context.Background(), model.VerifyCommand{ShopCode: "local-shop", ChallengeID: challenge.ID, Code: "123456"})
	if err != nil || got.ID != challenge.ID {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	stored, _ := repo.Get(context.Background(), challenge.ID)
	if stored.Status != model.StatusConsumed {
		t.Fatalf("status=%s", stored.Status)
	}
}

func TestVerifyRejectsWrongCodeWithoutDeletingChallenge(t *testing.T) {
	repo := &memoryRepository{
		shops:   map[string]struct{ merchantID, shopID int64 }{"local-shop": {2001, 3001}},
		records: map[string]model.Record{},
	}
	otp := newOTP(repo, &recordingNotifier{deliveries: []model.Delivery{{Channel: "SMS", Status: model.StatusSent}}})
	challenge, err := otp.Request(context.Background(), model.RequestCommand{ShopCode: "local-shop", Phone: "13800000000"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otp.Verify(context.Background(), model.VerifyCommand{ShopCode: "local-shop", ChallengeID: challenge.ID, Code: "000000"}); err != model.ErrInvalid {
		t.Fatalf("err=%v", err)
	}
	stored, _ := repo.Get(context.Background(), challenge.ID)
	if stored.Status != model.StatusPending || stored.AttemptCount != 1 {
		t.Fatalf("stored=%+v", stored)
	}
}
