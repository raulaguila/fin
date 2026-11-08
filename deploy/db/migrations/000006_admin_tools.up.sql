-- The audit log becomes readable through the API (audit:read).
INSERT INTO usr.usr_resource (code, description) VALUES ('audit', 'Audit log of administrative changes');
CREATE INDEX idx_usr_audit_log_actor ON usr.usr_audit_log (actor_id, created_at);

-- An administrator may end every session of another user.
ALTER TABLE usr.usr_session DROP CONSTRAINT chk_usr_session_revocation_reason;
ALTER TABLE usr.usr_session ADD CONSTRAINT chk_usr_session_revocation_reason CHECK (
    revocation_reason IN ('user_logout', 'token_reuse', 'password_changed', 'expired', 'account_disabled', 'admin_revoked')
);
