-- 000003_seed_rbac
-- Catalogo + papeis user (default, zero permissoes) e admin (*).

INSERT INTO usr.usr_permission (code, description)
VALUES
    ('users:read',       'Listar e obter usuarios'),
    ('users:write',      'Reservado; mutacao admin de perfil fora da v1'),
    ('roles:read',       'Listar e obter papeis'),
    ('roles:write',      'CRUD de papel e permissoes do papel'),
    ('roles:assign',     'Associar e desassociar papeis a usuarios'),
    ('permissions:read', 'Listar o catalogo')
ON CONFLICT (code) DO NOTHING;

INSERT INTO usr.usr_role (name, enabled, is_default)
VALUES ('user', true, true)
ON CONFLICT (name) DO NOTHING;

INSERT INTO usr.usr_role (name, enabled, is_default)
VALUES ('admin', true, false)
ON CONFLICT (name) DO NOTHING;

INSERT INTO usr.usr_role_permission (role_id, permission_code)
SELECT r.id, '*'
FROM usr.usr_role r
WHERE r.name = 'admin'
ON CONFLICT (role_id, permission_code) DO NOTHING;
