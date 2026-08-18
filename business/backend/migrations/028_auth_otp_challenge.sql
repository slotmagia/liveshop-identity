-- Identity auth owns shop-scoped login OTP challenges. The plaintext code is
-- never stored; Platform notification only delivers template variables.

CREATE TABLE identity_auth_otp_challenge (
  challenge_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  shop_code VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  phone VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  email VARCHAR(254) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT '',
  code_hash CHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  ttl_seconds INT NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  consumed_at DATETIME(3) NULL,
  PRIMARY KEY (challenge_id),
  KEY idx_identity_auth_otp_destination (shop_id, phone, email, status),
  CONSTRAINT fk_identity_auth_otp_merchant FOREIGN KEY (merchant_id)
    REFERENCES identity_merchant(merchant_id),
  CONSTRAINT fk_identity_auth_otp_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_auth_otp_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_auth_otp_destination CHECK (phone <> '' OR email <> ''),
  CONSTRAINT ck_identity_auth_otp_ttl CHECK (ttl_seconds = 300),
  CONSTRAINT ck_identity_auth_otp_status CHECK (status IN ('PENDING','CONSUMED','EXPIRED')),
  CONSTRAINT ck_identity_auth_otp_attempts CHECK (attempt_count >= 0 AND attempt_count <= 5),
  CONSTRAINT ck_identity_auth_otp_hash CHECK (code_hash REGEXP '^[0-9a-f]{64}$')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
