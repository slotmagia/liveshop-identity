CREATE TABLE identity_entitlement_projection_state (
  merchant_id BIGINT NOT NULL,
  entitlement_revision BIGINT NOT NULL,
  snapshot_digest BINARY(32) NOT NULL,
  source_updated_at DATETIME(3) NOT NULL,
  projected_at DATETIME(3) NOT NULL,
  PRIMARY KEY (merchant_id),
  CONSTRAINT ck_identity_entitlement_projection_revision CHECK (entitlement_revision > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Retire the former ownerless Identity rows. The one-time Platform handoff now
-- imports these facts into Subscription first; authorization stays closed
-- until the versioned Subscription snapshot is projected back here.
DELETE FROM identity_entitlement_projection;
UPDATE identity_authorization_domain
SET entitlement_revision=0
WHERE domain_type='MERCHANT';

ALTER TABLE identity_entitlement_projection
  ADD CONSTRAINT fk_identity_entitlement_projection_state
    FOREIGN KEY (merchant_id) REFERENCES identity_entitlement_projection_state(merchant_id) ON DELETE CASCADE;
