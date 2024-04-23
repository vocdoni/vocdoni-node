-- +goose Up
ALTER TABLE processes DROP COLUMN start_block;
ALTER TABLE processes DROP COLUMN end_block;
ALTER TABLE processes DROP COLUMN block_count;

