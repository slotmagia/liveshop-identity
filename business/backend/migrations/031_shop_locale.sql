-- Shop published locales are Identity facts. Platform localization is only
-- composed for completion percent. app_id/commercial_id are not identifiers.

CREATE TABLE identity_shop_locale (
  merchant_id BIGINT NOT NULL,
  shop_id BIGINT NOT NULL,
  locale VARCHAR(16) COLLATE utf8mb4_0900_as_cs NOT NULL,
  published TINYINT(1) NOT NULL DEFAULT 1,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (shop_id, locale),
  KEY idx_identity_shop_locale_merchant (merchant_id, shop_id, sort_order, locale),
  CONSTRAINT fk_identity_shop_locale_shop FOREIGN KEY (shop_id)
    REFERENCES identity_shop(shop_id),
  CONSTRAINT ck_identity_shop_locale_scope CHECK (merchant_id > 0 AND shop_id > 0),
  CONSTRAINT ck_identity_shop_locale_published CHECK (published IN (0,1)),
  CONSTRAINT ck_identity_shop_locale_sort CHECK (sort_order >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO identity_shop_locale (merchant_id, shop_id, locale, published, sort_order)
SELECT merchant_id, shop_id, IF(default_locale='', 'zh-CN', default_locale), 1, 0
FROM identity_shop
WHERE status <> 'CLOSED'
ON DUPLICATE KEY UPDATE published=VALUES(published);
