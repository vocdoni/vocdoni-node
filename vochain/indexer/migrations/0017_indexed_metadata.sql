-- +goose Up
-- Human readable metadata resolved from off-chain storage, so that list endpoints
-- can render a name without one detail request per row. Empty means "not resolved
-- (yet)", which clients treat as absent rather than as an empty title/name.
ALTER TABLE processes ADD COLUMN metadata_title TEXT NOT NULL DEFAULT '';

ALTER TABLE accounts ADD COLUMN name TEXT NOT NULL DEFAULT '';

ALTER TABLE accounts ADD COLUMN avatar TEXT NOT NULL DEFAULT '';

-- Backs the organization name filter. Note sqlite can only use this index for
-- anchored prefixes, and the filter matches anywhere in the name, so the index
-- helps the ordering and prefix cases rather than the general substring scan.
CREATE INDEX index_accounts_name
ON accounts(name);

-- +goose Down
DROP INDEX index_accounts_name;

ALTER TABLE accounts DROP COLUMN avatar;

ALTER TABLE accounts DROP COLUMN name;

ALTER TABLE processes DROP COLUMN metadata_title;
