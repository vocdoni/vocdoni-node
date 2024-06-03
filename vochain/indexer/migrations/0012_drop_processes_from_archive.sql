-- +goose Up
DELETE FROM processes WHERE from_archive = true;
ALTER TABLE processes DROP COLUMN from_archive;
