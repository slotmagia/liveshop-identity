CREATE TABLE IF NOT EXISTS subscription_authorization_import_ledger (
  import_id VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  source VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  schema_version INT NOT NULL,
  payload_sha256 CHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  export_row_count BIGINT NOT NULL,
  target_imported_row_count BIGINT NOT NULL,
  target_projection_digest CHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  target_subscription_instance VARCHAR(255) COLLATE utf8mb4_0900_as_cs NOT NULL,
  target_subscription_schema_version INT NOT NULL,
  receipt_key_id VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  receipt_json JSON NOT NULL,
  imported_at VARCHAR(35) COLLATE utf8mb4_0900_as_cs NOT NULL,
  PRIMARY KEY (import_id),
  UNIQUE KEY uq_subscription_authorization_import_digest (payload_sha256),
  CONSTRAINT ck_subscription_authorization_import_schema CHECK (schema_version = 1),
  CONSTRAINT ck_subscription_authorization_import_rows CHECK (export_row_count >= 0 AND target_imported_row_count >= 0),
  CONSTRAINT ck_subscription_authorization_import_target_schema CHECK (target_subscription_schema_version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
