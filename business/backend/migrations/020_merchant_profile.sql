-- Merchant self-service marketing opt-in flags for the merch profile page.
-- Existing contact_name remains a single field; first/last names are not split.

ALTER TABLE identity_merchant
    ADD COLUMN marketing_email_opt_in TINYINT(1) NOT NULL DEFAULT 0 AFTER contact_phone,
    ADD COLUMN marketing_sms_opt_in TINYINT(1) NOT NULL DEFAULT 0 AFTER marketing_email_opt_in;
