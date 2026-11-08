package shared

// Unprotected filters a query on usr.usr_user u down to users the API may manage.
// It must agree with domain.Role.Protected.
const Unprotected = `NOT EXISTS (
	SELECT 1 FROM usr.usr_user_role ur
	JOIN usr.usr_role r ON r.id = ur.role_id
	WHERE ur.user_id = u.id AND (
		r.is_superuser OR EXISTS (SELECT 1 FROM usr.usr_role_permission p WHERE p.role_id = r.id AND p.resource = '*')
	)
)`
