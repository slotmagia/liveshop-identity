-- Splits plan metadata writes from permission/quota policy writes. Existing
-- policy rows remain authoritative and start at revision 1.

ALTER TABLE subscription_plan
  ADD COLUMN policy_revision BIGINT NOT NULL DEFAULT 1 AFTER version,
  ADD CONSTRAINT ck_subscription_plan_policy_revision CHECK (policy_revision > 0);

CREATE TABLE subscription_plan_policy_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  plan_id BIGINT NOT NULL DEFAULT 0,
  response_revision BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
