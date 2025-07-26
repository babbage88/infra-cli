-- +goose Up
-- +goose StatementBegin
INSERT INTO public.health_check (id, status, check_type, created_at, last_modified)
 VALUES(gen_random_uuid(), 'healthy', 'Read', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), 
    (gen_random_uuid(), 'healthy', 'Delete', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM health_check WHERE check_type IN ('Read', 'Delete')
-- +goose StatementEnd
