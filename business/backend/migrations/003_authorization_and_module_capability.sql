-- Identity authorization is the single write path for roles, grants and data
-- scopes. Permission/contribution definitions remain owned by Platform
-- Registry and enter only through monotonic Inbox projections.

CREATE TABLE identity_registry_projection_state (
  singleton_id TINYINT NOT NULL,
  registry_revision BIGINT NOT NULL,
  snapshot_digest BINARY(32) NOT NULL,
  projected_at DATETIME(3) NOT NULL,
  PRIMARY KEY (singleton_id),
  CONSTRAINT ck_identity_registry_singleton CHECK (singleton_id = 1),
  CONSTRAINT ck_identity_registry_revision CHECK (registry_revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_permission_projection (
  permission_code VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
  module_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  name VARCHAR(191) NOT NULL,
  resource_code VARCHAR(160) COLLATE utf8mb4_0900_as_cs NOT NULL,
  action VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  description VARCHAR(500) NOT NULL,
  registry_revision BIGINT NOT NULL,
  active TINYINT(1) NOT NULL,
  PRIMARY KEY (permission_code),
  KEY idx_identity_permission_active (module_id, active, permission_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_contribution_projection (
  module_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  module_version VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  contribution_id VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  surface VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  capability_json JSON NOT NULL,
  registry_revision BIGINT NOT NULL,
  active TINYINT(1) NOT NULL,
  PRIMARY KEY (module_id, module_version, contribution_id),
  KEY idx_identity_contribution_active (surface, active, contribution_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_module_projection (
  module_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  module_version VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  module_name VARCHAR(191) NOT NULL,
  release_json JSON NOT NULL,
  registry_revision BIGINT NOT NULL,
  active TINYINT(1) NOT NULL,
  PRIMARY KEY (module_id, module_version),
  KEY idx_identity_module_active (active, module_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_authorization_domain (
  domain_type VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  domain_id BIGINT NOT NULL,
  revision BIGINT NOT NULL DEFAULT 1,
  entitlement_revision BIGINT NOT NULL DEFAULT 0,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (domain_type, domain_id),
  CONSTRAINT ck_identity_auth_domain_type CHECK (domain_type IN ('PLATFORM_ORG','MERCHANT')),
  CONSTRAINT ck_identity_auth_domain_revision CHECK (revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_authorization_role (
  domain_type VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  domain_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  code VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  name VARCHAR(191) NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  system_role TINYINT(1) NOT NULL DEFAULT 0,
  version BIGINT NOT NULL,
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY(domain_type,domain_id,role_id),
  UNIQUE KEY uq_identity_auth_role_code(domain_type,domain_id,code),
  CONSTRAINT fk_identity_auth_role_domain FOREIGN KEY(domain_type,domain_id) REFERENCES identity_authorization_domain(domain_type,domain_id),
  CONSTRAINT ck_identity_auth_role_status CHECK(status IN ('ACTIVE','DISABLED','DELETED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_authorization_role_permission (
  domain_type VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  domain_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  permission_code VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
  PRIMARY KEY(domain_type,domain_id,role_id,permission_code),
  CONSTRAINT fk_identity_auth_role_permission_role FOREIGN KEY(domain_type,domain_id,role_id) REFERENCES identity_authorization_role(domain_type,domain_id,role_id) ON DELETE CASCADE,
  CONSTRAINT fk_identity_auth_role_permission_catalog FOREIGN KEY(permission_code) REFERENCES identity_permission_projection(permission_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_authorization_role_scope (
  domain_type VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  domain_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  resource_code VARCHAR(160) COLLATE utf8mb4_0900_as_cs NOT NULL,
  scope_type VARCHAR(40) COLLATE utf8mb4_0900_as_cs NOT NULL,
  reference_json JSON NOT NULL,
  PRIMARY KEY(domain_type,domain_id,role_id,resource_code),
  CONSTRAINT fk_identity_auth_role_scope_role FOREIGN KEY(domain_type,domain_id,role_id) REFERENCES identity_authorization_role(domain_type,domain_id,role_id) ON DELETE CASCADE,
  CONSTRAINT ck_identity_auth_scope_type CHECK(scope_type IN ('SELF','CURRENT_ORG_UNIT','ORG_UNIT_SUBTREE','CURRENT_SHOP','ASSIGNED_SHOPS','DELEGATED_BUSINESS_SCOPE','CUSTOM_REFERENCE'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_subject_grant (
  grant_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  domain_type VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  domain_id BIGINT NOT NULL,
  subject VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  role_id BIGINT NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  access_version BIGINT NOT NULL,
  operation_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  active_marker TINYINT GENERATED ALWAYS AS(IF(status='ACTIVE',1,NULL)) STORED,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  revoked_at DATETIME(3) NULL,
  PRIMARY KEY(grant_id),
  UNIQUE KEY uq_identity_subject_active_role(domain_type,domain_id,subject,role_id,active_marker),
  KEY idx_identity_subject_grant(domain_type,domain_id,subject,status),
  CONSTRAINT fk_identity_subject_grant_role FOREIGN KEY(domain_type,domain_id,role_id) REFERENCES identity_authorization_role(domain_type,domain_id,role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_authorization_operation (
  operation_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  domain_type VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  domain_id BIGINT NOT NULL,
  subject VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  access_version BIGINT NOT NULL,
  request_hash VARBINARY(32) NOT NULL,
  completed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY(operation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_entitlement_projection (
  merchant_id BIGINT NOT NULL,
  permission_code VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  entitlement_revision BIGINT NOT NULL,
  PRIMARY KEY(merchant_id,permission_code),
  CONSTRAINT ck_identity_entitlement_status CHECK(status IN ('ACTIVE','REVOKED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Explicitly no seed permission codes: Registry is the only permission-definition owner.
