-- Each resource lists the actions its routes check, so a role cannot hold a
-- grant that does nothing (audit is read-only; users are disabled, never deleted).
ALTER TABLE usr.usr_resource ADD COLUMN actions INTEGER NOT NULL DEFAULT 15;

UPDATE usr.usr_resource SET actions = 2 WHERE code = 'audit';
UPDATE usr.usr_resource SET actions = 7 WHERE code = 'users';

UPDATE usr.usr_role_permission p SET actions = p.actions & r.actions
FROM usr.usr_resource r WHERE p.resource = r.code;
DELETE FROM usr.usr_role_permission WHERE actions = 0;
