CREATE TABLE IF NOT EXISTS subscription_permission_entitlement_state (
  merchant_id BIGINT NOT NULL,
  revision BIGINT NOT NULL,
  snapshot_digest BINARY(32) NOT NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (merchant_id),
  CONSTRAINT ck_subscription_permission_entitlement_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS subscription_permission_entitlement_item (
  merchant_id BIGINT NOT NULL,
  permission_code VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
  revision BIGINT NOT NULL,
  PRIMARY KEY (merchant_id, permission_code),
  CONSTRAINT fk_subscription_permission_entitlement_state
    FOREIGN KEY (merchant_id) REFERENCES subscription_permission_entitlement_state(merchant_id) ON DELETE CASCADE,
  CONSTRAINT ck_subscription_permission_entitlement_item_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS subscription_permission_entitlement_command (
  merchant_id BIGINT NOT NULL,
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  response_revision BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (merchant_id, command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
