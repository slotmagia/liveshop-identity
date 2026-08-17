CREATE TABLE IF NOT EXISTS subscription_quota_entitlement (
  app_id BIGINT NOT NULL,
  quota_code VARCHAR(96) NOT NULL,
  limit_value BIGINT NULL COMMENT 'NULL means explicitly unlimited',
  revision BIGINT NOT NULL,
  effective_from DATETIME(6) NOT NULL,
  effective_until DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (app_id, quota_code),
  CONSTRAINT chk_subscription_quota_limit CHECK (limit_value IS NULL OR limit_value > 0),
  CONSTRAINT chk_subscription_quota_revision CHECK (revision > 0),
  CONSTRAINT chk_subscription_quota_window CHECK (effective_until IS NULL OR effective_until > effective_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS subscription_quota_command (
  app_id BIGINT NOT NULL,
  command_key VARCHAR(128) NOT NULL,
  request_hash CHAR(64) NOT NULL,
  quota_code VARCHAR(96) NOT NULL,
  response_revision BIGINT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (app_id, command_key),
  KEY idx_subscription_quota_command_quota (app_id, quota_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
