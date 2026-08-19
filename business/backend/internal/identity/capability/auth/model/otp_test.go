package model

import (
	"errors"
	"testing"
	"time"
)

func TestRemainingResendSeconds(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	if got := RemainingResendSeconds(time.Time{}, now); got != 0 {
		t.Fatalf("empty last=%d", got)
	}
	if got := RemainingResendSeconds(now, now); got != ResendIntervalSeconds {
		t.Fatalf("just sent=%d", got)
	}
	if got := RemainingResendSeconds(now.Add(-59*time.Second), now); got != 1 {
		t.Fatalf("59s ago=%d", got)
	}
	if got := RemainingResendSeconds(now.Add(-time.Duration(ResendIntervalSeconds)*time.Second), now); got != 0 {
		t.Fatalf("exactly cooled=%d", got)
	}
	if got := RemainingResendSeconds(now.Add(-59*time.Second-100*time.Millisecond), now); got != 1 {
		t.Fatalf("fractional remaining=%d", got)
	}
}

func TestResendCooldownErrorUnwraps(t *testing.T) {
	err := &ResendCooldownError{ResendAfterSeconds: 12, NextSendAt: NextSendAt(time.Unix(1, 0))}
	if !errors.Is(err, ErrResendCooldown) {
		t.Fatalf("unwrap failed: %v", err)
	}
	if err.Error() != "login otp resend cooldown: 12" {
		t.Fatalf("error=%q", err.Error())
	}
}
