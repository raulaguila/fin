-- 000007_performance_indexes
-- usr_token.user_id: a FK sem indice varria a tabela por usuario apagado.
-- app.product.created_at: listagem sem ordenar a tabela inteira.
-- usr_login_attempt: contagens so de falhas que testaram senha. O predicado deve
-- ser igual ao de attempt/repository.go, ou o planner ignora os indices.

CREATE INDEX idx_usr_token_user_id ON usr.usr_token (user_id);

CREATE INDEX idx_app_product_created ON app.product (created_at DESC);

DROP INDEX usr.idx_usr_login_attempt_email_created;
DROP INDEX usr.idx_usr_login_attempt_ip_created;

CREATE INDEX idx_usr_login_attempt_email_counted
    ON usr.usr_login_attempt (email, created_at)
    WHERE success = false
      AND failure_kind NOT IN ('rate_limited_email', 'rate_limited_ip', 'account_locked');

CREATE INDEX idx_usr_login_attempt_ip_counted
    ON usr.usr_login_attempt (ip_address, created_at)
    WHERE success = false
      AND failure_kind NOT IN ('rate_limited_email', 'rate_limited_ip', 'account_locked');
