-- A consumed shop login OTP may be redeemed once by /auth/login.
-- The plaintext code is already gone; this column only binds the
-- resulting CUSTOMER session so the challenge cannot be replayed.

ALTER TABLE identity_auth_otp_challenge
  ADD COLUMN login_session_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NULL AFTER consumed_at,
  ADD UNIQUE KEY uq_identity_auth_otp_login_session (login_session_id);
