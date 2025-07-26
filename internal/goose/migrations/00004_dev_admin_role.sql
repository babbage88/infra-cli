-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    devuser_id uuid;
    dev_adminroleid uuid;
BEGIN
    -- Find the Admin roll uuid for the default Admin user
    SELECT id INTO dev_adminroleid FROM public.user_roles WHERE role_name = 'Admin';

    -- Find the user ID for the username of the default admin user.
-- +goose envsub on
SELECT id INTO devuser_id FROM public.users WHERE username = '${DEV_APP_USER}';
-- +goose envsub off

    -- Ensure the user exists first before creating the role mapping for default admin user.
    IF devuser_id IS NOT NULL THEN
        -- Map the user to the Admin role
        INSERT INTO public.user_role_mapping (user_id, role_id)
        VALUES (devuser_id, dev_adminroleid)
        ON CONFLICT DO NOTHING;
    ELSE
        RAISE NOTICE 'No DEV_APP_USER found. Ensure correct env var is set. No mapping created.';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE dev_adminroleid uuid;
BEGIN
    SELECT id INTO dev_adminroleid FROM public.user_roles WHERE role_name = 'Admin';
    -- Remove the user_role_mapping for the user "devuser" and Admin role (role_id = 999)
    DELETE FROM public.user_role_mapping
-- +goose envsub on
    WHERE user_id = (SELECT id FROM public.users WHERE username = '${DEV_APP_USER}')
-- +goose envsub off
      AND role_id = dev_adminroleid;
END $$;
-- +goose StatementEnd
