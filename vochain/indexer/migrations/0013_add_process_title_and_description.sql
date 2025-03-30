-- +goose Up
ALTER TABLE processes ADD COLUMN title TEXT NOT NULL DEFAULT '';
ALTER TABLE processes ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE processes DROP COLUMN description;
ALTER TABLE processes DROP COLUMN title;
