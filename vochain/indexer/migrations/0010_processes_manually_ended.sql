-- +goose Up
ALTER TABLE processes
ADD COLUMN manually_ended BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE processes
SET manually_ended = end_block < (start_block+block_count);
