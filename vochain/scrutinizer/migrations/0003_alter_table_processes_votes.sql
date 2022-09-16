-- +goose Up
ALTER TABLE vote_references ADD voter_id BLOB NOT NULL;

-- results columns; in badgerhold these were a separate table
ALTER TABLE processes ADD results_votes TEXT NOT NULL;
ALTER TABLE processes ADD results_weight TEXT NOT NULL;
ALTER TABLE processes ADD results_envelope_height INTEGER NOT NULL;
ALTER TABLE processes ADD results_signatures TEXT NOT NULL;
ALTER TABLE processes ADD results_block_height INTEGER NOT NULL;

-- +goose Down
ALTER TABLE vote_references DROP COLUMN voter_id;

ALTER TABLE processes DROP COLUMN results_votes;
ALTER TABLE processes DROP COLUMN results_weight;
ALTER TABLE processes DROP COLUMN results_envelope_height;
ALTER TABLE processes DROP COLUMN results_signatures;
ALTER TABLE processes DROP COLUMN results_block_height;
