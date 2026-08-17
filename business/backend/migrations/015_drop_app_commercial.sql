-- Drop app_id and commercial_id from Identity runtime tables. Shop isolation
-- is merchant_id + shop_id only. Historical 001-014 files stay unchanged.

-- Quota: re-key from app_id to merchant_id using the shop directory before
-- those columns disappear. Ambiguous app-to-merchant mappings fail closed.
ALTER TABLE subscription_quota_entitlement
  ADD COLUMN merchant_id BIGINT NULL AFTER app_id;
ALTER TABLE subscription_quota_command
  ADD COLUMN merchant_id BIGINT NULL AFTER app_id;

UPDATE subscription_quota_entitlement q
JOIN (
  SELECT app_id, MIN(merchant_id) AS merchant_id, COUNT(DISTINCT merchant_id) AS merchant_count
  FROM identity_shop
  GROUP BY app_id
) s ON s.app_id = q.app_id AND s.merchant_count = 1
SET q.merchant_id = s.merchant_id;

UPDATE subscription_quota_command q
JOIN (
  SELECT app_id, MIN(merchant_id) AS merchant_id, COUNT(DISTINCT merchant_id) AS merchant_count
  FROM identity_shop
  GROUP BY app_id
) s ON s.app_id = q.app_id AND s.merchant_count = 1
SET q.merchant_id = s.merchant_id;

UPDATE subscription_quota_entitlement q
JOIN identity_merchant_application ma ON ma.app_id = q.app_id AND ma.status = 'ACTIVE'
SET q.merchant_id = ma.merchant_id
WHERE q.merchant_id IS NULL
  AND (SELECT COUNT(DISTINCT merchant_id) FROM identity_merchant_application x WHERE x.app_id = q.app_id AND x.status = 'ACTIVE') = 1;

UPDATE subscription_quota_command q
JOIN identity_merchant_application ma ON ma.app_id = q.app_id AND ma.status = 'ACTIVE'
SET q.merchant_id = ma.merchant_id
WHERE q.merchant_id IS NULL
  AND (SELECT COUNT(DISTINCT merchant_id) FROM identity_merchant_application x WHERE x.app_id = q.app_id AND x.status = 'ACTIVE') = 1;

CREATE TEMPORARY TABLE identity_quota_rekey_guard (
  leftover INT NOT NULL,
  CONSTRAINT ck_identity_quota_rekey_guard CHECK (leftover = 0)
);
INSERT INTO identity_quota_rekey_guard
SELECT (
  SELECT COUNT(*) FROM subscription_quota_entitlement WHERE merchant_id IS NULL
) + (
  SELECT COUNT(*) FROM subscription_quota_command WHERE merchant_id IS NULL
);
DROP TEMPORARY TABLE identity_quota_rekey_guard;

ALTER TABLE subscription_quota_entitlement DROP PRIMARY KEY;
ALTER TABLE subscription_quota_entitlement DROP COLUMN app_id;
ALTER TABLE subscription_quota_entitlement MODIFY merchant_id BIGINT NOT NULL;
ALTER TABLE subscription_quota_entitlement ADD PRIMARY KEY (merchant_id, quota_code);
ALTER TABLE subscription_quota_entitlement
  ADD CONSTRAINT ck_subscription_quota_merchant CHECK (merchant_id > 0);

ALTER TABLE subscription_quota_command DROP PRIMARY KEY;
ALTER TABLE subscription_quota_command DROP KEY idx_subscription_quota_command_quota;
ALTER TABLE subscription_quota_command DROP COLUMN app_id;
ALTER TABLE subscription_quota_command MODIFY merchant_id BIGINT NOT NULL;
ALTER TABLE subscription_quota_command ADD PRIMARY KEY (merchant_id, command_key);
ALTER TABLE subscription_quota_command ADD KEY idx_subscription_quota_command_quota (merchant_id, quota_code);
ALTER TABLE subscription_quota_command
  ADD CONSTRAINT ck_subscription_quota_command_merchant CHECK (merchant_id > 0);

-- Credential namespace no longer includes app_id. APP namespaces become MERCHANT.
UPDATE identity_credential
SET namespace_type = 'MERCHANT', app_id = NULL
WHERE namespace_type = 'APP' AND merchant_id IS NOT NULL;

CREATE TEMPORARY TABLE identity_credential_rekey_guard (
  leftover INT NOT NULL,
  CONSTRAINT ck_identity_credential_rekey_guard CHECK (leftover = 0)
);
INSERT INTO identity_credential_rekey_guard
SELECT COUNT(*) FROM identity_credential
WHERE namespace_type = 'APP' OR (namespace_type = 'SHOP' AND (merchant_id IS NULL OR shop_id IS NULL));
DROP TEMPORARY TABLE identity_credential_rekey_guard;

ALTER TABLE identity_credential DROP INDEX uq_identity_credential_identifier;
ALTER TABLE identity_credential DROP COLUMN namespace_key;
ALTER TABLE identity_credential DROP CHECK ck_identity_credential_namespace;
ALTER TABLE identity_credential DROP COLUMN app_id;
ALTER TABLE identity_credential
  ADD COLUMN namespace_key VARCHAR(96) GENERATED ALWAYS AS (
    CONCAT(namespace_type, ':', COALESCE(merchant_id, 0), ':', COALESCE(shop_id, 0))
  ) STORED;
ALTER TABLE identity_credential
  ADD UNIQUE KEY uq_identity_credential_identifier (namespace_key, credential_kind, normalized_identifier);
ALTER TABLE identity_credential
  ADD CONSTRAINT ck_identity_credential_namespace CHECK (
    (namespace_type = 'GLOBAL' AND merchant_id IS NULL AND shop_id IS NULL) OR
    (namespace_type = 'MERCHANT' AND merchant_id IS NOT NULL AND shop_id IS NULL) OR
    (namespace_type = 'SHOP' AND merchant_id IS NOT NULL AND shop_id IS NOT NULL)
  );

ALTER TABLE identity_session DROP COLUMN selected_app_id;
ALTER TABLE identity_session DROP COLUMN selected_commercial_id;

ALTER TABLE identity_merchant_capability DROP CHECK ck_identity_merchant_capability_scope;
ALTER TABLE identity_merchant_capability DROP COLUMN app_id;
ALTER TABLE identity_merchant_capability DROP COLUMN commercial_id;
ALTER TABLE identity_merchant_capability
  ADD CONSTRAINT ck_identity_merchant_capability_scope CHECK (merchant_id > 0 AND shop_id > 0);

ALTER TABLE identity_merchant_capability_audit DROP COLUMN app_id;
ALTER TABLE identity_merchant_capability_audit DROP COLUMN commercial_id;

ALTER TABLE identity_customer_service_account DROP CHECK ck_identity_customer_service_scope;
ALTER TABLE identity_customer_service_account DROP COLUMN app_id;
ALTER TABLE identity_customer_service_account DROP COLUMN commercial_id;
ALTER TABLE identity_customer_service_account
  ADD CONSTRAINT ck_identity_customer_service_scope CHECK (merchant_id > 0 AND shop_id > 0);

DROP TABLE IF EXISTS identity_merchant_application;

ALTER TABLE identity_shop DROP INDEX uq_identity_shop_commercial;
ALTER TABLE identity_shop DROP CHECK ck_identity_shop_ids_v2;
ALTER TABLE identity_shop DROP COLUMN app_id;
ALTER TABLE identity_shop DROP COLUMN commercial_id;
ALTER TABLE identity_shop
  ADD CONSTRAINT ck_identity_shop_ids_v3 CHECK (shop_id > 0 AND merchant_id > 0);
