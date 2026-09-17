-- 000003_seed_rbac
-- Catalogo usr_resource + papeis user (default) e admin (is_superuser + resource=*).
--
-- Equivalencia documentacao (API pode expandir bits):
--   users:create       ('users',       1)
--   users:read         ('users',       2)
--   users:update       ('users',       4)
--   users:delete       ('users',       8)
--   roles:create       ('roles',       1)
--   roles:read         ('roles',       2)
--   roles:update       ('roles',       4)
--   roles:delete       ('roles',       8)
--   permissions:read   ('permissions', 2)
--   products:create    ('products',    1)
--   ... CRUD analogo
-- Seed admin: so '*'; grants granulares via API.

INSERT INTO usr.usr_resource (code, description)
VALUES
    ('users',       'Contas e perfil'),
    ('roles',       'Papeis RBAC'),
    ('permissions', 'Consulta de resources e mascaras'),
    ('products',    'Modulo exemplo CRUD')
ON CONFLICT (code) DO NOTHING;

INSERT INTO usr.usr_role (name, enabled, is_default, is_superuser)
VALUES ('user', true, true, false)
ON CONFLICT (name) DO NOTHING;

INSERT INTO usr.usr_role (name, enabled, is_default, is_superuser)
VALUES ('admin', true, false, true)
ON CONFLICT (name) DO NOTHING;

-- Garante is_superuser no admin se o papel ja existia sem a coluna preenchida
UPDATE usr.usr_role SET is_superuser = true WHERE name = 'admin';

INSERT INTO usr.usr_role_permission (role_id, resource, actions)
SELECT r.id, '*', 2147483647  -- 0x7FFFFFFF; todos os bits INT4 positivos
FROM usr.usr_role r
WHERE r.name = 'admin'
ON CONFLICT (role_id, resource) DO NOTHING;
