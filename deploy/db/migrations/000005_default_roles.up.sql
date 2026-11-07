-- Several roles may be default: activation gives a new account every enabled one.
DROP INDEX usr.uni_usr_role_one_default;

-- Reading the catalog is part of editing roles (roles:read), not a grant of its own.
DELETE FROM usr.usr_role_permission WHERE resource = 'permissions';
DELETE FROM usr.usr_resource WHERE code = 'permissions';
