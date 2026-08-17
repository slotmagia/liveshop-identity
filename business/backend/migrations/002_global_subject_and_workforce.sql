-- Identity v2 is the only runtime model. The v1 tables are retained only as
-- one-time migration evidence; application code must never read them. Unique
-- key failures deliberately stop migration when legacy rows cannot be mapped
-- without changing their meaning.

-- Legacy CUSTOMER credentials are scoped by (app_id, merchant_id). They can
-- become SHOP credentials only when that scope resolves to exactly one shop.
-- Run this preflight before the first RENAME so an ambiguous legacy database
-- is rejected without leaving a half-created v2 schema.
CREATE TEMPORARY TABLE identity_customer_shop_preflight (
    credential_id BIGINT NOT NULL PRIMARY KEY,
    shop_count INT NOT NULL,
    CONSTRAINT ck_identity_customer_shop_preflight CHECK (shop_count = 1)
);
INSERT INTO identity_customer_shop_preflight(credential_id,shop_count)
SELECT c.credential_id,COUNT(s.shop_id)
FROM identity_credential c
LEFT JOIN identity_shop s ON s.app_id=c.app_id AND s.merchant_id=c.merchant_id
WHERE c.realm='CUSTOMER'
GROUP BY c.credential_id;
DROP TEMPORARY TABLE identity_customer_shop_preflight;

RENAME TABLE
    identity_refresh_token TO identity_refresh_token_legacy_v1,
    identity_session TO identity_session_legacy_v1,
    identity_staff_shop TO identity_staff_shop_legacy_v1,
    identity_organization_member TO identity_organization_member_legacy_v1,
    identity_organization_unit TO identity_organization_unit_legacy_v1,
    identity_staff TO identity_staff_legacy_v1,
    identity_shop TO identity_shop_legacy_v1,
    identity_merchant TO identity_merchant_legacy_v1,
    identity_credential TO identity_credential_legacy_v1,
    identity_subject TO identity_subject_legacy_v1,
    identity_outbox TO identity_outbox_legacy_v1,
    identity_inbox TO identity_inbox_legacy_v1;

CREATE TABLE identity_subject (
    subject          VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    realm            VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    principal_type   VARCHAR(32)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    display_name     VARCHAR(128) NOT NULL DEFAULT '',
    legacy_uid       BIGINT       NULL,
    status           VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version          BIGINT       NOT NULL,
    created_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    closed_at        DATETIME(3)  NULL,
    PRIMARY KEY (subject),
    UNIQUE KEY uq_identity_subject_legacy_uid (realm, legacy_uid),
    CONSTRAINT ck_identity_subject_realm_v2 CHECK (realm IN ('PLATFORM', 'MERCHANT', 'CUSTOMER')),
    CONSTRAINT ck_identity_subject_principal CHECK (
        (realm = 'PLATFORM' AND principal_type = 'PLATFORM_OPERATOR') OR
        (realm = 'MERCHANT' AND principal_type IN ('MERCHANT_OWNER', 'MERCHANT_STAFF', 'SHOP_ANCHOR')) OR
        (realm = 'CUSTOMER' AND principal_type IN ('CUSTOMER', 'GUEST'))
    ),
    CONSTRAINT ck_identity_subject_status_v2 CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_subject_version_v2 CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_merchant (
    merchant_id    BIGINT       NOT NULL,
    name           VARCHAR(191) NOT NULL,
    external_id    VARCHAR(64)  NULL,
    contact_name   VARCHAR(128) NOT NULL DEFAULT '',
    contact_phone  VARCHAR(32)  NOT NULL DEFAULT '',
    status         VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version        BIGINT       NOT NULL,
    created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    closed_at      DATETIME(3)  NULL,
    PRIMARY KEY (merchant_id),
    CONSTRAINT ck_identity_merchant_id CHECK (merchant_id > 0),
    CONSTRAINT ck_identity_merchant_status_v2 CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_merchant_version_v2 CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_merchant_application (
    merchant_id BIGINT      NOT NULL,
    app_id      BIGINT      NOT NULL,
    status      VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
    version     BIGINT      NOT NULL,
    created_at  DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at  DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (merchant_id, app_id),
    CONSTRAINT fk_identity_merchant_application_merchant FOREIGN KEY (merchant_id)
        REFERENCES identity_merchant (merchant_id),
    CONSTRAINT ck_identity_merchant_application_id CHECK (app_id > 0),
    CONSTRAINT ck_identity_merchant_application_status CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_merchant_application_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_shop (
    shop_id         BIGINT       NOT NULL,
    merchant_id     BIGINT       NOT NULL,
    app_id          BIGINT       NOT NULL,
    commercial_id   BIGINT       NOT NULL,
    code             VARCHAR(32)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    subdomain        VARCHAR(191) NULL,
    name             VARCHAR(191) NOT NULL,
    default_locale   VARCHAR(16)  NOT NULL DEFAULT '',
    currency         VARCHAR(8)   COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT 'USD',
    status           VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version          BIGINT       NOT NULL,
    created_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at       DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    closed_at        DATETIME(3)  NULL,
    PRIMARY KEY (shop_id),
    UNIQUE KEY uq_identity_shop_commercial (app_id, commercial_id),
    UNIQUE KEY uq_identity_shop_code (code),
    UNIQUE KEY uq_identity_shop_subdomain (subdomain),
    KEY idx_identity_shop_merchant (merchant_id, status),
    CONSTRAINT fk_identity_shop_merchant_v2 FOREIGN KEY (merchant_id)
        REFERENCES identity_merchant (merchant_id),
    CONSTRAINT ck_identity_shop_ids_v2 CHECK (shop_id > 0 AND app_id > 0 AND commercial_id > 0),
    CONSTRAINT ck_identity_shop_status_v2 CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_shop_version_v2 CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_organization (
    organization_id   BIGINT       NOT NULL AUTO_INCREMENT,
    organization_type VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    merchant_id       BIGINT       NULL,
    name              VARCHAR(191) NOT NULL,
    status            VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version           BIGINT       NOT NULL,
    created_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    closed_at         DATETIME(3)  NULL,
    platform_slot TINYINT GENERATED ALWAYS AS (
        IF(organization_type = 'PLATFORM' AND status <> 'CLOSED', 1, NULL)
    ) STORED,
    merchant_slot BIGINT GENERATED ALWAYS AS (
        IF(organization_type = 'MERCHANT' AND status <> 'CLOSED', merchant_id, NULL)
    ) STORED,
    PRIMARY KEY (organization_id),
    UNIQUE KEY uq_identity_organization_platform_slot (platform_slot),
    UNIQUE KEY uq_identity_organization_merchant_slot (merchant_slot),
    CONSTRAINT fk_identity_organization_merchant_v2 FOREIGN KEY (merchant_id)
        REFERENCES identity_merchant (merchant_id),
    CONSTRAINT ck_identity_organization_type CHECK (organization_type IN ('PLATFORM', 'MERCHANT')),
    CONSTRAINT ck_identity_organization_owner CHECK (
        (organization_type = 'PLATFORM' AND merchant_id IS NULL) OR
        (organization_type = 'MERCHANT' AND merchant_id IS NOT NULL)
    ),
    CONSTRAINT ck_identity_organization_status_v2 CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_organization_version_v2 CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_organization_unit (
    organization_id       BIGINT       NOT NULL,
    organization_unit_id  BIGINT       NOT NULL,
    parent_unit_id         BIGINT       NULL,
    name                   VARCHAR(191) NOT NULL,
    status                 VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version                BIGINT       NOT NULL,
    created_at             DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at             DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (organization_id, organization_unit_id),
    KEY idx_identity_organization_unit_parent (organization_id, parent_unit_id),
    CONSTRAINT fk_identity_organization_unit_org FOREIGN KEY (organization_id)
        REFERENCES identity_organization (organization_id),
    CONSTRAINT fk_identity_organization_unit_parent FOREIGN KEY (organization_id, parent_unit_id)
        REFERENCES identity_organization_unit (organization_id, organization_unit_id),
    CONSTRAINT ck_identity_organization_unit_status CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_organization_unit_not_self CHECK (parent_unit_id IS NULL OR parent_unit_id <> organization_unit_id),
    CONSTRAINT ck_identity_organization_unit_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_workforce_member (
    member_id          BIGINT       NOT NULL AUTO_INCREMENT,
    organization_id    BIGINT       NOT NULL,
    merchant_id        BIGINT       NULL,
    subject            VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    member_type        VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    legacy_staff_id    BIGINT       NULL,
    status             VARCHAR(32)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    access_version     BIGINT       NOT NULL,
    created_at         DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at         DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at         DATETIME(3)  NULL,
    owner_slot BIGINT GENERATED ALWAYS AS (
        IF(member_type = 'OWNER' AND status <> 'REVOKED', organization_id, NULL)
    ) STORED,
    PRIMARY KEY (member_id),
    UNIQUE KEY uq_identity_workforce_subject_org (organization_id, subject),
    UNIQUE KEY uq_identity_workforce_legacy_staff (merchant_id, legacy_staff_id),
    UNIQUE KEY uq_identity_workforce_owner_slot (owner_slot),
    KEY idx_identity_workforce_merchant (merchant_id, status),
    CONSTRAINT fk_identity_workforce_org FOREIGN KEY (organization_id)
        REFERENCES identity_organization (organization_id),
    CONSTRAINT fk_identity_workforce_merchant FOREIGN KEY (merchant_id)
        REFERENCES identity_merchant (merchant_id),
    CONSTRAINT fk_identity_workforce_subject FOREIGN KEY (subject)
        REFERENCES identity_subject (subject),
    CONSTRAINT ck_identity_workforce_type CHECK (member_type IN ('OPERATOR', 'OWNER', 'STAFF', 'ANCHOR')),
    CONSTRAINT ck_identity_workforce_owner CHECK (
        (member_type = 'OPERATOR' AND merchant_id IS NULL) OR
        (member_type IN ('OWNER', 'STAFF', 'ANCHOR') AND merchant_id IS NOT NULL)
    ),
    CONSTRAINT ck_identity_workforce_status CHECK (status IN ('ACTIVE', 'SUSPENDED', 'REVOKED')),
    CONSTRAINT ck_identity_workforce_access_version CHECK (access_version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_organization_membership (
    organization_id      BIGINT      NOT NULL,
    organization_unit_id BIGINT      NOT NULL,
    member_id            BIGINT      NOT NULL,
    is_primary           TINYINT(1)  NOT NULL DEFAULT 0,
    status               VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
    created_at           DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at           DATETIME(3) NULL,
    primary_slot BIGINT GENERATED ALWAYS AS (
        IF(is_primary = 1 AND status = 'ACTIVE', member_id, NULL)
    ) STORED,
    PRIMARY KEY (organization_id, organization_unit_id, member_id),
    UNIQUE KEY uq_identity_organization_membership_primary (primary_slot),
    CONSTRAINT fk_identity_organization_membership_unit FOREIGN KEY (organization_id, organization_unit_id)
        REFERENCES identity_organization_unit (organization_id, organization_unit_id),
    CONSTRAINT fk_identity_organization_membership_member FOREIGN KEY (member_id)
        REFERENCES identity_workforce_member (member_id),
    CONSTRAINT ck_identity_organization_membership_status CHECK (status IN ('ACTIVE', 'REVOKED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_member_shop (
    member_id       BIGINT      NOT NULL,
    shop_id         BIGINT      NOT NULL,
    assignment_kind VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
    status          VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
    created_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    revoked_at      DATETIME(3) NULL,
    active_assignment_slot VARCHAR(96) GENERATED ALWAYS AS (
        IF(status = 'ACTIVE', CONCAT(member_id, ':', shop_id, ':', assignment_kind), NULL)
    ) STORED,
    anchor_member_slot BIGINT GENERATED ALWAYS AS (
        IF(status = 'ACTIVE' AND assignment_kind = 'ANCHOR', member_id, NULL)
    ) STORED,
    anchor_shop_slot BIGINT GENERATED ALWAYS AS (
        IF(status = 'ACTIVE' AND assignment_kind = 'ANCHOR', shop_id, NULL)
    ) STORED,
    PRIMARY KEY (member_id, shop_id, assignment_kind),
    UNIQUE KEY uq_identity_member_shop_active (active_assignment_slot),
    UNIQUE KEY uq_identity_member_anchor (anchor_member_slot),
    UNIQUE KEY uq_identity_shop_anchor (anchor_shop_slot),
    KEY idx_identity_member_shop_shop (shop_id, status),
    CONSTRAINT fk_identity_member_shop_member FOREIGN KEY (member_id)
        REFERENCES identity_workforce_member (member_id),
    CONSTRAINT fk_identity_member_shop_shop FOREIGN KEY (shop_id)
        REFERENCES identity_shop (shop_id),
    CONSTRAINT ck_identity_member_shop_kind CHECK (assignment_kind IN ('OPERATE', 'ANCHOR')),
    CONSTRAINT ck_identity_member_shop_status CHECK (status IN ('ACTIVE', 'REVOKED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_credential (
    credential_id        BIGINT       NOT NULL AUTO_INCREMENT,
    subject              VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    namespace_type       VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    app_id               BIGINT       NULL,
    merchant_id          BIGINT       NULL,
    shop_id              BIGINT       NULL,
    credential_kind      VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    normalized_identifier VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
    secret_hash          VARCHAR(255) COLLATE utf8mb4_0900_as_cs NULL,
    status               VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    failed_login_count   INT          NOT NULL DEFAULT 0,
    locked_until         DATETIME(3)  NULL,
    verified_at          DATETIME(3)  NULL,
    version              BIGINT       NOT NULL,
    created_at           DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at           DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    namespace_key VARCHAR(96) GENERATED ALWAYS AS (
        CONCAT(namespace_type, ':', COALESCE(app_id, 0), ':', COALESCE(merchant_id, 0), ':', COALESCE(shop_id, 0))
    ) STORED,
    PRIMARY KEY (credential_id),
    UNIQUE KEY uq_identity_credential_identifier (namespace_key, credential_kind, normalized_identifier),
    KEY idx_identity_credential_subject (subject, status),
    CONSTRAINT fk_identity_credential_subject_v2 FOREIGN KEY (subject)
        REFERENCES identity_subject (subject),
    CONSTRAINT ck_identity_credential_namespace CHECK (
        (namespace_type = 'GLOBAL' AND app_id IS NULL AND merchant_id IS NULL AND shop_id IS NULL) OR
        (namespace_type = 'APP' AND app_id IS NOT NULL AND merchant_id IS NULL AND shop_id IS NULL) OR
        (namespace_type = 'MERCHANT' AND merchant_id IS NOT NULL AND shop_id IS NULL) OR
        (namespace_type = 'SHOP' AND app_id IS NOT NULL AND merchant_id IS NOT NULL AND shop_id IS NOT NULL)
    ),
    CONSTRAINT ck_identity_credential_kind_v2 CHECK (credential_kind IN ('USERNAME', 'EMAIL', 'PHONE')),
    CONSTRAINT ck_identity_credential_status_v2 CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_credential_failures_v2 CHECK (failed_login_count >= 0),
    CONSTRAINT ck_identity_credential_version_v2 CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_session (
    session_id          VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    session_family_id   VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    subject             VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    selected_organization_id BIGINT NULL,
    selected_merchant_id BIGINT NULL,
    selected_app_id     BIGINT NULL,
    selected_commercial_id BIGINT NULL,
    selected_shop_id    BIGINT NULL,
    context_version     BIGINT NOT NULL,
    authentication_level VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT 'PASSWORD',
    device_name         VARCHAR(255) NOT NULL DEFAULT '',
    ip_address          VARCHAR(64)  NOT NULL DEFAULT '',
    user_agent          VARCHAR(500) NOT NULL DEFAULT '',
    status              VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    created_at          DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    last_refreshed_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    expires_at          DATETIME(3)  NOT NULL,
    revoked_at          DATETIME(3)  NULL,
    revoke_reason       VARCHAR(64)  NULL,
    PRIMARY KEY (session_id),
    KEY idx_identity_session_subject (subject, status),
    KEY idx_identity_session_family (session_family_id, status),
    CONSTRAINT fk_identity_session_subject_v2 FOREIGN KEY (subject)
        REFERENCES identity_subject (subject),
    CONSTRAINT fk_identity_session_shop FOREIGN KEY (selected_shop_id)
        REFERENCES identity_shop (shop_id),
    CONSTRAINT ck_identity_session_status CHECK (status IN ('ACTIVE', 'REVOKED', 'EXPIRED')),
    CONSTRAINT ck_identity_session_context_version CHECK (context_version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_refresh_token (
    token_hash VARBINARY(32) NOT NULL,
    session_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
    status     VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
    issued_at  DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    expires_at DATETIME(3) NOT NULL,
    used_at    DATETIME(3) NULL,
    PRIMARY KEY (token_hash),
    KEY idx_identity_refresh_session (session_id, status),
    CONSTRAINT fk_identity_refresh_session_v2 FOREIGN KEY (session_id)
        REFERENCES identity_session (session_id) ON DELETE CASCADE,
    CONSTRAINT ck_identity_refresh_status_v2 CHECK (status IN ('ACTIVE', 'USED', 'REVOKED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_idempotency (
    command_name   VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    idempotency_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    request_hash   VARBINARY(32) NOT NULL,
    result_json    JSON         NULL,
    created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    completed_at   DATETIME(3)  NULL,
    PRIMARY KEY (command_name, idempotency_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_outbox (
    event_id          VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    aggregate_type    VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    aggregate_id      VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
    aggregate_version BIGINT       NOT NULL,
    event_type        VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    payload_json      JSON         NOT NULL,
    created_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    published_at      DATETIME(3)  NULL,
    attempts          INT          NOT NULL DEFAULT 0,
    last_error        VARCHAR(500) NULL,
    PRIMARY KEY (event_id),
    KEY idx_identity_outbox_pending (published_at, created_at),
    CONSTRAINT ck_identity_outbox_version_v2 CHECK (aggregate_version > 0),
    CONSTRAINT ck_identity_outbox_attempts_v2 CHECK (attempts >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_inbox (
    consumer_name     VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
    event_id          VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
    aggregate_id      VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
    aggregate_version BIGINT      NOT NULL,
    processed_at      DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (consumer_name, event_id),
    KEY idx_identity_inbox_aggregate (consumer_name, aggregate_id, aggregate_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Migrate only rows whose old semantics are unambiguous. Conflicting global
-- subjects, merchant IDs, shops or credentials make a unique constraint fail,
-- which is intentional: an operator must provide an explicit mapping instead
-- of this migration guessing.
INSERT INTO identity_subject
    (subject, realm, principal_type, display_name, legacy_uid, status, version, created_at, updated_at, closed_at)
SELECT DISTINCT s.subject, s.realm,
    CASE
        WHEN s.realm = 'PLATFORM' THEN 'PLATFORM_OPERATOR'
        WHEN s.realm = 'CUSTOMER' THEN 'CUSTOMER'
        WHEN st.subject IS NOT NULL THEN 'MERCHANT_STAFF'
        ELSE 'MERCHANT_OWNER'
    END,
    COALESCE(st.display_name, ''), s.legacy_uid, s.status, s.version, s.created_at, s.updated_at, s.closed_at
FROM identity_subject_legacy_v1 s
LEFT JOIN identity_staff_legacy_v1 st
  ON st.realm = s.realm AND st.app_id = s.app_id AND st.merchant_id = s.merchant_id AND st.subject = s.subject;

INSERT INTO identity_merchant
    (merchant_id, name, external_id, contact_name, contact_phone, status, version, created_at, updated_at, closed_at)
SELECT DISTINCT merchant_id, name, external_id, contact_name, contact_phone, status, version, created_at, updated_at, closed_at
FROM identity_merchant_legacy_v1;

INSERT INTO identity_merchant_application (merchant_id, app_id, status, version, created_at, updated_at)
SELECT DISTINCT merchant_id, app_id, status, version, created_at, updated_at
FROM identity_merchant_legacy_v1;

INSERT IGNORE INTO identity_merchant_application (merchant_id, app_id, status, version)
SELECT DISTINCT merchant_id, app_id, 'ACTIVE', 1 FROM identity_shop_legacy_v1;

INSERT INTO identity_shop
    (shop_id, merchant_id, app_id, commercial_id, code, subdomain, name, default_locale, currency, status, version, created_at, updated_at, closed_at)
SELECT shop_id, merchant_id, app_id, commercial_id, code, subdomain, name, default_locale, currency, status, version, created_at, updated_at, closed_at
FROM identity_shop_legacy_v1;

INSERT INTO identity_organization (organization_type, merchant_id, name, status, version)
SELECT 'PLATFORM', NULL, 'Platform', 'ACTIVE', 1
WHERE EXISTS (SELECT 1 FROM identity_subject WHERE realm = 'PLATFORM');

INSERT INTO identity_organization (organization_type, merchant_id, name, status, version, created_at, updated_at, closed_at)
SELECT 'MERCHANT', merchant_id, name, status, version, created_at, updated_at, closed_at
FROM identity_merchant;

INSERT INTO identity_organization_unit
    (organization_id, organization_unit_id, parent_unit_id, name, status, version, created_at, updated_at)
SELECT o.organization_id, u.organization_unit_id, u.parent_unit_id, u.name, u.status, u.version, u.created_at, u.updated_at
FROM identity_organization_unit_legacy_v1 u
JOIN identity_organization o ON o.organization_type = 'MERCHANT' AND o.merchant_id = u.merchant_id;

-- A merchant without an old organization tree receives one explicit root.
INSERT INTO identity_organization_unit
    (organization_id, organization_unit_id, parent_unit_id, name, status, version)
SELECT o.organization_id, 1, NULL, o.name, 'ACTIVE', 1
FROM identity_organization o
WHERE NOT EXISTS (
    SELECT 1 FROM identity_organization_unit u WHERE u.organization_id = o.organization_id
);

INSERT INTO identity_workforce_member
    (organization_id, merchant_id, subject, member_type, legacy_staff_id, status, access_version, created_at, updated_at, revoked_at)
SELECT o.organization_id, st.merchant_id, st.subject, 'STAFF', st.staff_id,
    CASE st.status WHEN 'ACTIVE' THEN 'ACTIVE' WHEN 'DISABLED' THEN 'SUSPENDED' ELSE 'REVOKED' END,
    st.version, st.created_at, st.updated_at, st.closed_at
FROM identity_staff_legacy_v1 st
JOIN identity_organization o ON o.organization_type = 'MERCHANT' AND o.merchant_id = st.merchant_id;

INSERT INTO identity_workforce_member
    (organization_id, merchant_id, subject, member_type, status, access_version, created_at, updated_at, revoked_at)
SELECT DISTINCT o.organization_id, s.merchant_id, s.subject, 'OWNER',
    CASE s.status WHEN 'ACTIVE' THEN 'ACTIVE' WHEN 'DISABLED' THEN 'SUSPENDED' ELSE 'REVOKED' END,
    s.version, s.created_at, s.updated_at, s.closed_at
FROM identity_subject_legacy_v1 s
JOIN identity_organization o ON o.organization_type = 'MERCHANT' AND o.merchant_id = s.merchant_id
LEFT JOIN identity_staff_legacy_v1 st
  ON st.realm = s.realm AND st.app_id = s.app_id AND st.merchant_id = s.merchant_id AND st.subject = s.subject
WHERE s.realm = 'MERCHANT' AND st.subject IS NULL;

INSERT INTO identity_workforce_member
    (organization_id, merchant_id, subject, member_type, status, access_version, created_at, updated_at, revoked_at)
SELECT o.organization_id, NULL, s.subject, 'OPERATOR',
    CASE s.status WHEN 'ACTIVE' THEN 'ACTIVE' WHEN 'DISABLED' THEN 'SUSPENDED' ELSE 'REVOKED' END,
    s.version, s.created_at, s.updated_at, s.closed_at
FROM identity_subject s
JOIN identity_organization o ON o.organization_type = 'PLATFORM'
WHERE s.realm = 'PLATFORM';

INSERT INTO identity_organization_membership
    (organization_id, organization_unit_id, member_id, is_primary, status, created_at)
SELECT o.organization_id, om.organization_unit_id, wm.member_id, om.is_primary, 'ACTIVE', om.created_at
FROM identity_organization_member_legacy_v1 om
JOIN identity_organization o ON o.organization_type = 'MERCHANT' AND o.merchant_id = om.merchant_id
JOIN identity_workforce_member wm ON wm.organization_id = o.organization_id AND wm.legacy_staff_id = om.staff_id;

INSERT INTO identity_member_shop (member_id, shop_id, assignment_kind, status, created_at)
SELECT wm.member_id, ss.shop_id, 'OPERATE', 'ACTIVE', ss.created_at
FROM identity_staff_shop_legacy_v1 ss
JOIN identity_workforce_member wm ON wm.merchant_id = ss.merchant_id AND wm.legacy_staff_id = ss.staff_id;

INSERT INTO identity_credential
    (credential_id, subject, namespace_type, app_id, merchant_id, shop_id, credential_kind,
     normalized_identifier, secret_hash, status, failed_login_count, locked_until, verified_at,
     version, created_at, updated_at)
SELECT c.credential_id, c.subject,
    CASE WHEN c.realm IN ('PLATFORM', 'MERCHANT') THEN 'GLOBAL' ELSE 'SHOP' END,
    CASE WHEN c.realm IN ('PLATFORM', 'MERCHANT') THEN NULL ELSE c.app_id END,
    CASE WHEN c.realm IN ('PLATFORM', 'MERCHANT') THEN NULL ELSE c.merchant_id END,
    CASE WHEN c.realm IN ('PLATFORM', 'MERCHANT') THEN NULL ELSE s.shop_id END,
    c.credential_kind, LOWER(TRIM(c.identifier)), c.secret_hash, c.status,
    c.failed_login_count, c.locked_until, c.verified_at, c.version, c.created_at, c.updated_at
FROM identity_credential_legacy_v1 c
LEFT JOIN identity_shop s ON c.realm='CUSTOMER' AND s.app_id=c.app_id AND s.merchant_id=c.merchant_id;

INSERT INTO identity_session
    (session_id, session_family_id, subject, context_version, device_name, ip_address, user_agent,
     status, created_at, last_refreshed_at, expires_at, revoked_at, revoke_reason)
SELECT session_id, session_id, subject, 1, device_name, ip_address, user_agent,
    IF(revoked_at IS NULL, 'ACTIVE', 'REVOKED'), created_at, last_refreshed_at, expires_at, revoked_at, revoke_reason
FROM identity_session_legacy_v1;

INSERT INTO identity_refresh_token (token_hash, session_id, status, issued_at, expires_at, used_at)
SELECT token_hash, session_id, status, issued_at, expires_at, used_at
FROM identity_refresh_token_legacy_v1;

INSERT INTO identity_outbox
    (event_id, aggregate_type, aggregate_id, aggregate_version, event_type, payload_json,
     created_at, published_at, attempts, last_error)
SELECT event_id, aggregate_type, aggregate_id, aggregate_version, event_type, payload_json,
       created_at, published_at, attempts, last_error
FROM identity_outbox_legacy_v1;

INSERT INTO identity_inbox (consumer_name, event_id, aggregate_id, aggregate_version, processed_at)
SELECT consumer_name, event_id, event_id, 1, processed_at
FROM identity_inbox_legacy_v1;
