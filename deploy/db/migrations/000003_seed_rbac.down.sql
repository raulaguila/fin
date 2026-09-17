DELETE FROM usr.usr_role_permission
WHERE resource = '*'
  AND role_id IN (SELECT id FROM usr.usr_role WHERE name = 'admin');

DELETE FROM usr.usr_role WHERE name IN ('user', 'admin');

DELETE FROM usr.usr_resource
WHERE code IN ('users', 'roles', 'permissions', 'products');
