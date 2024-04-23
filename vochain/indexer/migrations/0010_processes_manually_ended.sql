-- +goose Up
ALTER TABLE processes
ADD COLUMN manually_ended BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE processes
SET manually_ended = end_block < (start_block+block_count);

ALTER TABLE processes DROP COLUMN start_block;
ALTER TABLE processes DROP COLUMN end_block;
ALTER TABLE processes DROP COLUMN block_count;
