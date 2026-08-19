DELETE FROM usr.usr_role_permission
WHERE permission_code = '*'
  AND role_id IN (SELECT id FROM usr.usr_role WHERE name = 'admin');

DELETE FROM usr.usr_role WHERE name IN ('user', 'admin');

DELETE FROM usr.usr_permission
WHERE code IN (
    'users:read',
    'users:write',
    'roles:read',
    'roles:write',
    'roles:assign',
    'permissions:read'
);
