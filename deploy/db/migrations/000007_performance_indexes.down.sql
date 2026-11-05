DROP INDEX usr.idx_usr_login_attempt_ip_counted;
DROP INDEX usr.idx_usr_login_attempt_email_counted;

CREATE INDEX idx_usr_login_attempt_email_created ON usr.usr_login_attempt (email, created_at DESC);
CREATE INDEX idx_usr_login_attempt_ip_created ON usr.usr_login_attempt (ip_address, created_at DESC) WHERE ip_address IS NOT NULL;

DROP INDEX app.idx_app_product_created;

DROP INDEX usr.idx_usr_token_user_id;
