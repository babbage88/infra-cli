-- +goose Up
-- +goose StatementBegin
-- public.role_permissions_view source
CREATE OR REPLACE VIEW public.role_permissions_view
AS SELECT ur.id AS "RoleId",
    ur.role_name AS "Role",
    ap.id AS "PermissionId",
    ap.permission_name AS "Permission"
   FROM user_roles ur
     LEFT JOIN role_permission_mapping rpm ON rpm.role_id = ur.id
     LEFT JOIN app_permissions ap ON rpm.permission_id = ap.id
  WHERE ur.enabled = true AND rpm.enabled = true
  GROUP BY ur.id, ur.role_name, ap.id, ap.permission_name
  ORDER BY ur.id, ap.id;
-- +goose StatementEnd

-- +goose StatementBegin
-- public.user_permissions_view source

CREATE OR REPLACE VIEW public.user_permissions_view
AS SELECT u.id AS "UserId",
    u.username AS "Username",
    ap.id AS "PermissionId",
    ap.permission_name AS "Permission",
    ur.role_name AS "Role",
    urm.last_modified AS "LastModified"
   FROM user_role_mapping urm
     LEFT JOIN user_roles ur ON ur.id = urm.role_id
     LEFT JOIN users u ON u.id = urm.user_id
     LEFT JOIN role_permission_mapping rpm ON rpm.role_id = urm.role_id
     LEFT JOIN app_permissions ap ON ap.id = rpm.permission_id
  WHERE ur.enabled = true
  ORDER BY u.id;
-- +goose StatementEnd

-- +goose StatementBegin
-- public.user_roles_active source

CREATE OR REPLACE VIEW public.user_roles_active
AS SELECT user_roles.id AS "RoleId",
    user_roles.role_name AS "RoleName",
    user_roles.role_description AS "RoleDescription",
    user_roles.created_at AS "CreatedAt",
    user_roles.last_modified AS "LastModified",
    user_roles.enabled AS "Enabled",
    user_roles.is_deleted AS "IsDeleted"
   FROM user_roles
  WHERE user_roles.is_deleted IS FALSE;
-- +goose StatementEnd

-- +goose StatementBegin
-- public.users_with_roles source

CREATE OR REPLACE VIEW public.users_with_roles AS
SELECT u.id,
       u.username,
       u.password,
       u.email,
       COALESCE(array_agg(ur.role_name::text) FILTER (WHERE ur.role_name IS NOT NULL), ARRAY['None'::text]) AS roles,
       COALESCE(array_agg(urm.role_id::uuid) FILTER (WHERE urm.role_id IS NOT NULL), '{}'::uuid[]) AS role_ids,
       u.created_at,
       u.last_modified,
       u.enabled,
       u.is_deleted
FROM users u
LEFT JOIN user_role_mapping urm ON u.id = urm.user_id AND urm.enabled = true
LEFT JOIN user_roles ur ON urm.role_id = ur.id
WHERE u.is_deleted = false
GROUP BY u.id, u.username, u.password, u.email, u.created_at, u.last_modified, u.enabled, u.is_deleted;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW role_permissions_view;
DROP VIEW user_permissions_view;
DROP VIEW user_roles_active;
DROP VIEW users_with_roles;
-- +goose StatementEnd
