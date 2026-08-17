-- Platform operators are not subscription tenants. Their non-zero capability
-- boundary version is tracked separately from merchant entitlement revisions.
ALTER TABLE identity_authorization_domain
  ADD COLUMN platform_boundary_revision BIGINT NOT NULL DEFAULT 0 AFTER entitlement_revision;

ALTER TABLE identity_authorization_role_scope
  DROP CHECK ck_identity_auth_scope_type,
  ADD CONSTRAINT ck_identity_auth_scope_type CHECK(scope_type IN ('ALL','SELF','CURRENT_ORG_UNIT','ORG_UNIT_SUBTREE','CURRENT_SHOP','ASSIGNED_SHOPS','DELEGATED_BUSINESS_SCOPE','CUSTOM_REFERENCE'));

-- The handoff ledger is written in the same transaction as imported IAM facts.
-- A digest can be accepted only once; the stored signed receipt makes retries
-- byte-for-byte convergent and lets Platform prove the target committed first.
CREATE TABLE identity_authorization_import_ledger (
  import_id VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  source VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  schema_version INT NOT NULL,
  payload_sha256 CHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  row_count BIGINT NOT NULL,
  target_identity_instance VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  target_identity_schema_version INT NOT NULL,
  receipt_key_id VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  receipt_json JSON NOT NULL,
  imported_at DATETIME(3) NOT NULL,
  PRIMARY KEY(import_id),
  UNIQUE KEY uq_identity_authorization_import_digest(payload_sha256),
  CONSTRAINT ck_identity_authorization_import_schema CHECK(schema_version = 1),
  CONSTRAINT ck_identity_authorization_import_rows CHECK(row_count >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_authorization_bootstrap_ledger (
  bootstrap_id VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  manifest_sha256 CHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  manifest_json JSON NOT NULL,
  completed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY(bootstrap_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
