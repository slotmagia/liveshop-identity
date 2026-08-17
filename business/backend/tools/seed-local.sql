-- Local development identities only. Production must provision subjects and
-- credentials through the controlled Identity administration workflow.
START TRANSACTION;

INSERT INTO identity_subject
    (subject, realm, principal_type, display_name, status, version)
VALUES
    ('platform-admin', 'PLATFORM', 'PLATFORM_OPERATOR', 'Local Platform Admin', 'ACTIVE', 1),
    ('merchant-admin', 'MERCHANT', 'MERCHANT_OWNER', 'Local Merchant Owner', 'ACTIVE', 1),
    ('customer-local', 'CUSTOMER', 'CUSTOMER', 'Local Customer', 'ACTIVE', 1)
ON DUPLICATE KEY UPDATE subject = VALUES(subject);

INSERT INTO identity_merchant
    (merchant_id, name, external_id, status, version)
VALUES
    (2001, 'Local Merchant', 'local-merchant-2001', 'ACTIVE', 1)
ON DUPLICATE KEY UPDATE merchant_id = VALUES(merchant_id);

INSERT INTO identity_shop
    (shop_id, merchant_id, code, subdomain, name,
     default_locale, currency, status, version)
VALUES
    (3001, 2001, 'local-shop', 'local-shop', 'Local Shop',
     'zh-CN', 'CNY', 'ACTIVE', 1)
ON DUPLICATE KEY UPDATE shop_id = VALUES(shop_id);

INSERT INTO identity_organization
    (organization_id, organization_type, merchant_id, name, status, version)
VALUES
    (1, 'PLATFORM', NULL, 'Platform', 'ACTIVE', 1),
    (2, 'MERCHANT', 2001, 'Local Merchant', 'ACTIVE', 1)
ON DUPLICATE KEY UPDATE organization_id = VALUES(organization_id);

INSERT INTO identity_organization_unit
    (organization_id, organization_unit_id, parent_unit_id, name, status, version)
VALUES
    (1, 1, NULL, 'Platform', 'ACTIVE', 1),
    (2, 1, NULL, 'Local Merchant', 'ACTIVE', 1)
ON DUPLICATE KEY UPDATE organization_id = VALUES(organization_id);

INSERT INTO identity_workforce_member
    (member_id, organization_id, merchant_id, subject, member_type, status, access_version)
VALUES
    (1, 1, NULL, 'platform-admin', 'OPERATOR', 'ACTIVE', 1),
    (2, 2, 2001, 'merchant-admin', 'OWNER', 'ACTIVE', 1)
ON DUPLICATE KEY UPDATE member_id = VALUES(member_id);

INSERT INTO identity_organization_membership
    (organization_id, organization_unit_id, member_id, is_primary, status)
VALUES
    (1, 1, 1, TRUE, 'ACTIVE'),
    (2, 1, 2, TRUE, 'ACTIVE')
ON DUPLICATE KEY UPDATE member_id = VALUES(member_id);

INSERT INTO identity_credential
    (subject, namespace_type, merchant_id, shop_id, credential_kind, normalized_identifier,
     secret_hash, status, verified_at, version)
VALUES
    ('platform-admin', 'GLOBAL', NULL, NULL, 'USERNAME', 'admin',
     '$2a$10$pTTHqeOZYp6l498R88woIersxL4lvkC7oUQy7Ya5/rNzygbuoERh.',
     'ACTIVE', CURRENT_TIMESTAMP(3), 1),
    ('merchant-admin', 'GLOBAL', NULL, NULL, 'EMAIL', 'merch@sufeipay.com',
     '$2a$10$.leH53r6exPtubObzDWRyOOIBGAGuNvngw7jDzW7SnXJh1ByWlriS',
     'ACTIVE', CURRENT_TIMESTAMP(3), 1),
    ('customer-local', 'SHOP', 2001, 3001, 'USERNAME', 'customer',
     '$2a$10$.leH53r6exPtubObzDWRyOOIBGAGuNvngw7jDzW7SnXJh1ByWlriS',
     'ACTIVE', CURRENT_TIMESTAMP(3), 1)
ON DUPLICATE KEY UPDATE credential_id = credential_id;

INSERT INTO identity_visitor_risk
    (visitor_risk_id, merchant_id, shop_id, visitor_id, score, level, status, version, updated_at)
VALUES
    (1, 2001, 3001, 'v-1001', 88, 'HIGH', 'RESTRICTED', 2, '2026-08-18 10:20:00.000'),
    (2, 2001, 3001, 'v-1002', 55, 'MEDIUM', 'WATCH', 1, '2026-08-18 10:05:00.000')
ON DUPLICATE KEY UPDATE score = VALUES(score), level = VALUES(level), status = VALUES(status), version = VALUES(version);

INSERT INTO identity_risk_event
    (event_id, merchant_id, shop_id, visitor_id, nickname, room_id, reason,
     score_before, score_after_decay, score_delta, score_after, created_at)
VALUES
    (1, 2001, 3001, 'v-1001', 'Ada', 9001, 'spam', 20, 18, 30, 48, '2026-08-18 10:00:00.000'),
    (2, 2001, 3001, 'v-1002', 'Bob', 9001, 'abuse', 40, 36, 19, 55, '2026-08-18 10:05:00.000'),
    (3, 2001, 3001, 'v-1001', 'Ada', 9002, 'flood', 48, 44, 44, 88, '2026-08-18 10:20:00.000')
ON DUPLICATE KEY UPDATE event_id = VALUES(event_id);

COMMIT;
