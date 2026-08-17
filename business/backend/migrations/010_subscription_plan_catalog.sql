-- Migrates the legacy subscription-role catalog into an Identity-owned plan
-- aggregate. Permission definitions remain Platform Registry facts; this table
-- stores only the complete set selected by a plan.

CREATE TABLE subscription_plan_guard (
  singleton_id TINYINT NOT NULL,
  version BIGINT NOT NULL DEFAULT 1,
  PRIMARY KEY (singleton_id),
  CONSTRAINT ck_subscription_plan_guard_singleton CHECK (singleton_id = 1),
  CONSTRAINT ck_subscription_plan_guard_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO subscription_plan_guard(singleton_id, version) VALUES(1, 1);

CREATE TABLE subscription_plan (
  plan_id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(64) COLLATE utf8mb4_0900_as_cs NOT NULL,
  name VARCHAR(191) NOT NULL,
  level INT NOT NULL DEFAULT 0,
  price_minor BIGINT NOT NULL DEFAULT 0,
  duration_days INT NOT NULL DEFAULT 30,
  description TEXT NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  version BIGINT NOT NULL,
  default_slot TINYINT GENERATED ALWAYS AS (
    CASE WHEN is_default = 1 AND status = 'ACTIVE' THEN 1 ELSE NULL END
  ) STORED,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (plan_id),
  UNIQUE KEY uq_subscription_plan_code (code),
  UNIQUE KEY uq_subscription_plan_active_default (default_slot),
  KEY idx_subscription_plan_list (status, sort_order, level, plan_id),
  CONSTRAINT ck_subscription_plan_price CHECK (price_minor >= 0),
  CONSTRAINT ck_subscription_plan_duration CHECK (duration_days >= 0),
  CONSTRAINT ck_subscription_plan_status CHECK (status IN ('ACTIVE','DISABLED','RETIRED')),
  CONSTRAINT ck_subscription_plan_version CHECK (version > 0),
  CONSTRAINT ck_subscription_plan_default_status CHECK (is_default = 0 OR status = 'ACTIVE')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE subscription_plan_permission (
  plan_id BIGINT NOT NULL,
  permission_code VARCHAR(191) COLLATE utf8mb4_0900_as_cs NOT NULL,
  PRIMARY KEY (plan_id, permission_code),
  CONSTRAINT fk_subscription_plan_permission_plan
    FOREIGN KEY (plan_id) REFERENCES subscription_plan(plan_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE subscription_plan_quota (
  plan_id BIGINT NOT NULL,
  quota_code VARCHAR(96) COLLATE utf8mb4_0900_as_cs NOT NULL,
  limit_value BIGINT NULL COMMENT 'NULL means explicitly unlimited',
  PRIMARY KEY (plan_id, quota_code),
  CONSTRAINT fk_subscription_plan_quota_plan
    FOREIGN KEY (plan_id) REFERENCES subscription_plan(plan_id) ON DELETE RESTRICT,
  CONSTRAINT ck_subscription_plan_quota_limit CHECK (limit_value IS NULL OR limit_value > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE subscription_plan_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  plan_id BIGINT NOT NULL DEFAULT 0,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Preserve the three legacy catalog entries. The old system had no proven
-- Catalog product cap, so the target records explicit unlimited product quota
-- instead of inventing limits or relying on a missing row.
INSERT INTO subscription_plan
  (code,name,level,price_minor,duration_days,description,is_default,sort_order,status,version)
VALUES
  ('free','免费版',0,0,30,'',1,1,'ACTIVE',1),
  ('standard','标准版',1,0,30,'',0,2,'ACTIVE',1),
  ('premium','高级版',2,0,30,'',0,3,'ACTIVE',1);

INSERT INTO subscription_plan_quota(plan_id, quota_code, limit_value)
SELECT plan_id, 'catalog.products', NULL FROM subscription_plan;
