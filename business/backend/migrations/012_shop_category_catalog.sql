-- Identity owns the platform-maintained shop business-category directory.
-- Category codes are stable references used by shops; rows are retired rather
-- than deleted so historical and active shop references never become orphaned.

CREATE TABLE identity_shop_category (
  category_id BIGINT NOT NULL AUTO_INCREMENT,
  code VARCHAR(32) COLLATE utf8mb4_0900_as_cs NOT NULL,
  name VARCHAR(64) NOT NULL,
  icon VARCHAR(16) NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (category_id),
  UNIQUE KEY uq_identity_shop_category_code (code),
  KEY idx_identity_shop_category_list (status, sort_order, category_id),
  CONSTRAINT ck_identity_shop_category_sort CHECK (sort_order >= 0),
  CONSTRAINT ck_identity_shop_category_status CHECK (status IN ('ACTIVE','DISABLED','RETIRED')),
  CONSTRAINT ck_identity_shop_category_version CHECK (version > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE identity_shop
  ADD COLUMN category_code VARCHAR(32) COLLATE utf8mb4_0900_as_cs NULL AFTER currency,
  ADD KEY idx_identity_shop_category_code (category_code),
  ADD CONSTRAINT fk_identity_shop_category
    FOREIGN KEY (category_code) REFERENCES identity_shop_category(code) ON UPDATE RESTRICT ON DELETE RESTRICT;

CREATE TABLE identity_shop_category_command (
  command_key VARCHAR(128) COLLATE utf8mb4_0900_as_cs NOT NULL,
  request_hash BINARY(32) NOT NULL,
  category_id BIGINT NOT NULL DEFAULT 0,
  response_version BIGINT NOT NULL DEFAULT 0,
  response_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (command_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO identity_shop_category(code,name,icon,sort_order,status,version) VALUES
  ('apparel','服装服饰','👗',1,'ACTIVE',1),
  ('beauty','美妆个护','💄',2,'ACTIVE',1),
  ('electronics','3C数码','📱',3,'ACTIVE',1),
  ('home','家居家纺','🛋️',4,'ACTIVE',1),
  ('food','食品生鲜','🍎',5,'ACTIVE',1),
  ('mother_baby','母婴用品','🍼',6,'ACTIVE',1),
  ('jewelry','珠宝配饰','💍',7,'ACTIVE',1),
  ('sports','运动户外','⚽',8,'ACTIVE',1),
  ('other','其它','📦',99,'ACTIVE',1);
