UPDATE usr.usr_session SET revocation_reason = 'account_disabled' WHERE revocation_reason = 'admin_revoked';
ALTER TABLE usr.usr_session DROP CONSTRAINT chk_usr_session_revocation_reason;
ALTER TABLE usr.usr_session ADD CONSTRAINT chk_usr_session_revocation_reason CHECK (
    revocation_reason IN ('user_logout', 'token_reuse', 'password_changed', 'expired', 'account_disabled')
);

DROP INDEX usr.idx_usr_audit_log_actor;
DELETE FROM usr.usr_role_permission WHERE resource = 'audit';
DELETE FROM usr.usr_resource WHERE code = 'audit';
