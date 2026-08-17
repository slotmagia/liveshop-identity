CREATE TABLE IF NOT EXISTS identity_subject (
    realm       VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    app_id      BIGINT       NOT NULL,
    merchant_id BIGINT       NOT NULL,
    subject     VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    legacy_uid  BIGINT       NULL,
    status      VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version     BIGINT       NOT NULL,
    created_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    closed_at   DATETIME(3)  NULL,
    PRIMARY KEY (realm, app_id, merchant_id, subject),
    UNIQUE KEY uq_identity_subject_legacy_uid (realm, app_id, merchant_id, legacy_uid),
    CONSTRAINT ck_identity_subject_realm CHECK (realm IN ('PLATFORM', 'MERCHANT', 'CUSTOMER')),
    CONSTRAINT ck_identity_subject_status CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_subject_scope CHECK (app_id > 0 AND merchant_id > 0),
    CONSTRAINT ck_identity_subject_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_credential (
    credential_id      BIGINT       NOT NULL AUTO_INCREMENT,
    realm              VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    app_id             BIGINT       NOT NULL,
    merchant_id        BIGINT       NOT NULL,
    subject            VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    credential_kind    VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    identifier         VARCHAR(191) NOT NULL,
    secret_hash        VARCHAR(255) COLLATE utf8mb4_0900_as_cs NULL,
    status             VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    failed_login_count INT          NOT NULL DEFAULT 0,
    locked_until       DATETIME(3)  NULL,
    verified_at        DATETIME(3)  NULL,
    version            BIGINT       NOT NULL,
    created_at         DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at         DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (credential_id),
    UNIQUE KEY uq_identity_credential_identifier (realm, app_id, merchant_id, credential_kind, identifier),
    KEY idx_identity_credential_subject (realm, app_id, merchant_id, subject),
    CONSTRAINT fk_identity_credential_subject FOREIGN KEY (realm, app_id, merchant_id, subject)
        REFERENCES identity_subject (realm, app_id, merchant_id, subject),
    CONSTRAINT ck_identity_credential_kind CHECK (credential_kind IN ('USERNAME', 'EMAIL', 'PHONE')),
    CONSTRAINT ck_identity_credential_status CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_credential_failures CHECK (failed_login_count >= 0),
    CONSTRAINT ck_identity_credential_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_merchant (
    app_id         BIGINT       NOT NULL,
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
    PRIMARY KEY (app_id, merchant_id),
    CONSTRAINT ck_identity_merchant_status CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_merchant_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_shop (
    app_id         BIGINT       NOT NULL,
    merchant_id    BIGINT       NOT NULL,
    shop_id        BIGINT       NOT NULL,
    commercial_id  BIGINT       NOT NULL,
    code            VARCHAR(32)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    subdomain       VARCHAR(191) NULL,
    name            VARCHAR(191) NOT NULL,
    default_locale  VARCHAR(16)  NOT NULL DEFAULT '',
    currency        VARCHAR(8)   COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT 'USD',
    status          VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version         BIGINT       NOT NULL,
    created_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at      DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    closed_at       DATETIME(3)  NULL,
    PRIMARY KEY (app_id, merchant_id, shop_id),
    UNIQUE KEY uq_identity_shop_commercial (app_id, commercial_id),
    UNIQUE KEY uq_identity_shop_code (code),
    UNIQUE KEY uq_identity_shop_subdomain (subdomain),
    CONSTRAINT fk_identity_shop_merchant FOREIGN KEY (app_id, merchant_id)
        REFERENCES identity_merchant (app_id, merchant_id),
    CONSTRAINT ck_identity_shop_ids CHECK (shop_id > 0 AND commercial_id > 0),
    CONSTRAINT ck_identity_shop_status CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_shop_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_staff (
    realm        VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL DEFAULT 'MERCHANT',
    app_id       BIGINT       NOT NULL,
    merchant_id  BIGINT       NOT NULL,
    staff_id     BIGINT       NOT NULL,
    subject      VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    status       VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version      BIGINT       NOT NULL,
    created_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    closed_at    DATETIME(3)  NULL,
    PRIMARY KEY (app_id, merchant_id, staff_id),
    UNIQUE KEY uq_identity_staff_subject (app_id, merchant_id, subject),
    CONSTRAINT fk_identity_staff_merchant FOREIGN KEY (app_id, merchant_id)
        REFERENCES identity_merchant (app_id, merchant_id),
    CONSTRAINT fk_identity_staff_subject FOREIGN KEY (realm, app_id, merchant_id, subject)
        REFERENCES identity_subject (realm, app_id, merchant_id, subject),
    CONSTRAINT ck_identity_staff_realm CHECK (realm = 'MERCHANT'),
    CONSTRAINT ck_identity_staff_status CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_staff_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_organization_unit (
    app_id                 BIGINT       NOT NULL,
    merchant_id            BIGINT       NOT NULL,
    organization_unit_id   BIGINT       NOT NULL,
    parent_unit_id         BIGINT       NULL,
    name                   VARCHAR(191) NOT NULL,
    status                 VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    version                BIGINT       NOT NULL,
    created_at             DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at             DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (app_id, merchant_id, organization_unit_id),
    KEY idx_identity_organization_parent (app_id, merchant_id, parent_unit_id),
    CONSTRAINT fk_identity_organization_merchant FOREIGN KEY (app_id, merchant_id)
        REFERENCES identity_merchant (app_id, merchant_id),
    CONSTRAINT fk_identity_organization_parent FOREIGN KEY (app_id, merchant_id, parent_unit_id)
        REFERENCES identity_organization_unit (app_id, merchant_id, organization_unit_id),
    CONSTRAINT ck_identity_organization_status CHECK (status IN ('ACTIVE', 'DISABLED', 'CLOSED')),
    CONSTRAINT ck_identity_organization_not_self CHECK (parent_unit_id IS NULL OR parent_unit_id <> organization_unit_id),
    CONSTRAINT ck_identity_organization_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_organization_member (
    app_id                BIGINT      NOT NULL,
    merchant_id           BIGINT      NOT NULL,
    organization_unit_id  BIGINT      NOT NULL,
    staff_id               BIGINT      NOT NULL,
    is_primary             TINYINT(1)  NOT NULL DEFAULT 0,
    created_at             DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    primary_marker TINYINT GENERATED ALWAYS AS (IF(is_primary = 1, 1, NULL)) STORED,
    PRIMARY KEY (app_id, merchant_id, organization_unit_id, staff_id),
    UNIQUE KEY uq_identity_staff_primary_unit (app_id, merchant_id, staff_id, primary_marker),
    CONSTRAINT fk_identity_organization_member_unit FOREIGN KEY (app_id, merchant_id, organization_unit_id)
        REFERENCES identity_organization_unit (app_id, merchant_id, organization_unit_id),
    CONSTRAINT fk_identity_organization_member_staff FOREIGN KEY (app_id, merchant_id, staff_id)
        REFERENCES identity_staff (app_id, merchant_id, staff_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_staff_shop (
    app_id       BIGINT      NOT NULL,
    merchant_id  BIGINT      NOT NULL,
    staff_id     BIGINT      NOT NULL,
    shop_id      BIGINT      NOT NULL,
    created_at   DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (app_id, merchant_id, staff_id, shop_id),
    KEY idx_identity_staff_shop_shop (app_id, merchant_id, shop_id),
    CONSTRAINT fk_identity_staff_shop_staff FOREIGN KEY (app_id, merchant_id, staff_id)
        REFERENCES identity_staff (app_id, merchant_id, staff_id),
    CONSTRAINT fk_identity_staff_shop_shop FOREIGN KEY (app_id, merchant_id, shop_id)
        REFERENCES identity_shop (app_id, merchant_id, shop_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_session (
    session_id        VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    realm             VARCHAR(16)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    app_id            BIGINT       NOT NULL,
    merchant_id       BIGINT       NOT NULL,
    subject           VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    device_name       VARCHAR(255) NOT NULL DEFAULT '',
    ip_address        VARCHAR(64)  NOT NULL DEFAULT '',
    user_agent        VARCHAR(500) NOT NULL DEFAULT '',
    created_at        DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    last_refreshed_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    expires_at        DATETIME(3)  NOT NULL,
    revoked_at        DATETIME(3)  NULL,
    revoke_reason     VARCHAR(64)  NULL,
    PRIMARY KEY (session_id),
    KEY idx_identity_session_subject (realm, app_id, merchant_id, subject),
    CONSTRAINT fk_identity_session_subject FOREIGN KEY (realm, app_id, merchant_id, subject)
        REFERENCES identity_subject (realm, app_id, merchant_id, subject)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_refresh_token (
    token_hash VARBINARY(32) NOT NULL,
    session_id VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
    status     VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
    issued_at  DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    expires_at DATETIME(3) NOT NULL,
    used_at    DATETIME(3) NULL,
    PRIMARY KEY (token_hash),
    KEY idx_identity_refresh_session (session_id),
    CONSTRAINT fk_identity_refresh_session FOREIGN KEY (session_id)
        REFERENCES identity_session (session_id) ON DELETE CASCADE,
    CONSTRAINT ck_identity_refresh_status CHECK (status IN ('ACTIVE', 'USED', 'REVOKED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_outbox (
    event_id       VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    aggregate_type VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    aggregate_id   VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
    aggregate_version BIGINT    NOT NULL,
    event_type     VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
    payload_json   JSON         NOT NULL,
    created_at     DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    published_at   DATETIME(3)  NULL,
    attempts       INT          NOT NULL DEFAULT 0,
    last_error     VARCHAR(500) NULL,
    PRIMARY KEY (event_id),
    KEY idx_identity_outbox_pending (published_at, created_at),
    CONSTRAINT ck_identity_outbox_version CHECK (aggregate_version > 0),
    CONSTRAINT ck_identity_outbox_attempts CHECK (attempts >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS identity_inbox (
    consumer_name VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    event_id      VARCHAR(64)  COLLATE utf8mb4_0900_as_cs NOT NULL,
    processed_at  DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (consumer_name, event_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
