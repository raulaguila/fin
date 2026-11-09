-- A notification names its kind and parameters, so each client shows it in the
-- user's language. title and message stay as the English fallback.
ALTER TABLE usr.usr_notification
    ADD COLUMN kind   TEXT  NULL,
    ADD COLUMN params JSONB NOT NULL DEFAULT '{}';
