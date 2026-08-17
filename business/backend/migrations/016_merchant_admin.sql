-- Merchant admin command ledger and subscription assignment for the
-- admin merchants page. Historical identity_merchant / identity_shop
-- tables stay the fact source; this migration only adds write ledgers.

CREATE TABLE identity_merchant_command (
    command_key      VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    request_hash     BINARY(32)   NOT NULL,
    merchant_id      BIGINT       NULL,
    response_version BIGINT       NOT NULL DEFAULT 0,
    response_json    JSON         NULL,
    created_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    completed_at     DATETIME(3)  NULL,
    PRIMARY KEY (command_key),
    KEY idx_identity_merchant_command_merchant (merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE subscription_merchant_assignment (
    merchant_id  BIGINT      NOT NULL,
    plan_id      BIGINT      NOT NULL,
    expires_at   DATETIME(3) NULL,
    version      BIGINT      NOT NULL,
    created_at   DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (merchant_id),
    KEY idx_subscription_merchant_assignment_plan (plan_id),
    CONSTRAINT ck_subscription_merchant_assignment_ids CHECK (merchant_id > 0 AND plan_id > 0),
    CONSTRAINT ck_subscription_merchant_assignment_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE subscription_merchant_assignment_command (
    command_key      VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    request_hash     BINARY(32)   NOT NULL,
    merchant_id      BIGINT       NULL,
    response_version BIGINT       NOT NULL DEFAULT 0,
    response_json    JSON         NULL,
    created_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    completed_at     DATETIME(3)  NULL,
    PRIMARY KEY (command_key),
    KEY idx_subscription_merchant_assignment_command_merchant (merchant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
