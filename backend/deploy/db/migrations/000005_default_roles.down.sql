INSERT INTO usr.usr_resource (code, description) VALUES ('permissions', 'Resources and action masks');

-- Keep the oldest default so the single-default index can come back.
UPDATE usr.usr_role SET is_default = false
WHERE is_default AND id <> (SELECT id FROM usr.usr_role WHERE is_default ORDER BY created_at, id LIMIT 1);
CREATE UNIQUE INDEX uni_usr_role_one_default ON usr.usr_role (is_default) WHERE is_default;
