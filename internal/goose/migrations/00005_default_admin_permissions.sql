-- +goose Up
-- +goose StatementBegin
-- Create a temporary table to store role and user IDs
CREATE TEMP TABLE temp_admin_info (
    dev_adminroleid uuid,
    devuser_id uuid
);

-- Insert Admin role ID and default Admin user ID into the temporary table
INSERT INTO temp_admin_info (dev_adminroleid, devuser_id)
SELECT 
    (SELECT id FROM public.user_roles WHERE role_name = 'Admin'),
-- +goose envsub on
    (SELECT id FROM public.users WHERE username = '${DEV_APP_USER}');
-- +goose envsub off
-- +goose StatementEnd

-- +goose StatementBegin
-- Insert permissions
INSERT INTO public.app_permissions (permission_name, permission_description) VALUES
    ('CreateRole', 'Permission to Create User Roles'),
    ('AlterRole', 'Alter Role properties and Permissions to Create User Roles'),
    ('CreatePermission', 'Create Permission'),
    ('AlterPermission', 'Alter Permission properties'),
    ('DeleteUser', 'Delete Users'),
    ('DeleteRole', 'Delete Roles'),
    ('DeletePermission', 'Delete Permissions')
ON CONFLICT (permission_name) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
-- Map permissions to the Admin role
INSERT INTO public.role_permission_mapping (role_id, permission_id)
SELECT t.dev_adminroleid, p.id
FROM public.app_permissions p
JOIN temp_admin_info t ON TRUE
WHERE p.permission_name IN ('DeleteUser', 'DeleteRole', 'DeletePermission',
                            'CreateRole', 'AlterRole', 'CreatePermission', 'AlterPermission');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Create a temporary table to store permission IDs
CREATE TEMP TABLE temp_permissions AS
SELECT id FROM public.app_permissions
WHERE permission_name IN ('DeleteUser', 'DeleteRole', 'DeletePermission',
                          'CreateRole', 'AlterRole', 'CreatePermission', 'AlterPermission');
-- +goose StatementEnd

-- +goose StatementBegin
-- Create a temporary table to store role ID
CREATE TEMP TABLE temp_admin_info (
    dev_adminroleid uuid
);

-- Insert the Admin role ID
INSERT INTO temp_admin_info (dev_adminroleid)
SELECT id FROM public.user_roles WHERE role_name = 'Admin';
-- +goose StatementEnd

-- +goose StatementBegin
-- Delete from role_permission_mapping using the temp tables
DELETE FROM public.role_permission_mapping
WHERE role_id IN (SELECT dev_adminroleid FROM temp_admin_info)
AND permission_id IN (SELECT id FROM temp_permissions);
-- +goose StatementEnd

-- +goose StatementBegin
-- Delete the permissions that were inserted in the Up migration
DELETE FROM public.app_permissions
WHERE id IN (SELECT id FROM temp_permissions);
-- +goose StatementEnd
