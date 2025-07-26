-- +goose Up
-- +goose StatementBegin
-- +goose envsub on
-- Creating go-infra app/api user for development/testing
 INSERT INTO public.users
 (id, username, "password", email, created_at, last_modified)
 VALUES(gen_random_uuid(), 'devuser', '${DEV_APP_USER_PW}', 'devuser@test.trahan.dev', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
-- +goose envsub off
-- +goose StatementEnd

-- +goose StatementBegin
-- Generate the UUID for the Admin role
-- Insert permissions if they do not exist
-- Insert role-permission mappings
WITH admin_role AS (
    INSERT INTO public.user_roles (id, role_name, role_description)
    VALUES (gen_random_uuid(), 'Admin', 'Administrator role with all permissions')
    ON CONFLICT (role_name) DO NOTHING
    RETURNING id
),
permissions AS (

    INSERT INTO public.app_permissions (permission_name, permission_description)
    VALUES
        ('CreateUser', 'Permission to create users'),
        ('AlterUser', 'Permission to alter users'),
        ('CreateShare', 'Permission to create databases')
    ON CONFLICT (permission_name) DO NOTHING
    RETURNING id, permission_name
)
INSERT INTO public.role_permission_mapping (role_id, permission_id)
SELECT COALESCE((SELECT id FROM admin_role), (SELECT id FROM public.user_roles WHERE role_name = 'Admin')),
       p.id
FROM public.app_permissions p
WHERE p.permission_name IN ('CreateUser', 'AlterUser', 'CreateShare');
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DELETE FROM public.users WHERE "username" = 'devuser';
DELETE FROM public.user_roles WHERE role_name = 'Admin';
DELETE FROM public.app_permissions WHERE permission_name IN ('CreateUser', 'AlterUser', 'CreateShare');
-- +goose StatementEnd

