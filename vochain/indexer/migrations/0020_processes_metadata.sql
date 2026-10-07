-- +goose Up
-- The hash of the raw metadata document referenced by the metadata URI, as
-- committed on chain by the process creator (issue #1479). Empty means no hash
-- was committed. Processes indexed before this migration read as empty until
-- their next update or a db rebuild, since a migration cannot backfill from state.
ALTER TABLE processes ADD COLUMN metadata_hash BLOB NOT NULL DEFAULT x'';

-- Every metadata version a process has had: the one it was created with and each
-- SET_PROCESS_METADATA after it, so clients can audit the changes made during the
-- vote. (block_height, block_index) locates the transaction that set it.
CREATE TABLE process_metadata_history (
  process_id    BLOB NOT NULL,
  block_height  INTEGER NOT NULL,
  block_index   INTEGER NOT NULL,
  time          DATETIME NOT NULL,
  metadata      TEXT NOT NULL,
  metadata_hash BLOB NOT NULL,
  PRIMARY KEY (process_id, block_height, block_index)
);

-- Until now the metadata of a process could not change, so the current one is the
-- version it was created with. The creating NewProcessTx is located by the process
-- ID it carries, set by the node before indexing it: processId is the first field
-- of Process, so its 32 bytes follow the first 0x0a 0x20 (field 1, length 32) of
-- the protobuf encoding. That pair cannot occur earlier: raw_tx is a marshaled
-- Tx, encoded in field order as 12 <len> 08 <txtype> 10 <nonce> 1a <len> 0a 20,
-- where no tag is 0x0a, varint bytes other than the last have the high bit set,
-- and the last one is followed by a tag, never by 0x20 (a Process length ending
-- in 0x0a gives 0a 0a 20, which still matches at the right offset). Extracting
-- it once per tx keeps this linear. A tx without it, as indexed by a block
-- reindex which does not execute txs, may still match 32 bytes further on, but
-- those are no process ID and the join drops them. A process whose tx is not
-- found gets block_height 0 and block_index -1, so the rows that fell back can
-- be counted with WHERE block_index = -1.
INSERT INTO process_metadata_history (process_id, block_height, block_index, time, metadata, metadata_hash)
WITH created AS (
	SELECT substr(raw_tx, instr(raw_tx, x'0a20') + 2, 32) AS process_id,
		MIN(block_height) AS block_height, block_index
	FROM transactions
	WHERE type = 'newProcess' AND instr(raw_tx, x'0a20') > 0
	GROUP BY 1
)
SELECT p.id,
	COALESCE(c.block_height, 0),
	COALESCE(c.block_index, -1),
	p.creation_time, p.metadata, x''
FROM processes AS p
LEFT JOIN created AS c ON c.process_id = p.id
WHERE p.metadata != '';

-- +goose Down
DROP TABLE process_metadata_history;

ALTER TABLE processes DROP COLUMN metadata_hash;
