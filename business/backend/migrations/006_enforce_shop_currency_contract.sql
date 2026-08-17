ALTER TABLE identity_shop
    ADD CONSTRAINT ck_identity_shop_currency_iso4217
        CHECK (currency REGEXP '^[A-Z]{3}$');
