-- +goose Up
CREATE TABLE processes (
  id           BLOB NOT NULL PRIMARY KEY, -- TODO: also store as hex for fast search?
  entity_id    BLOB NOT NULL, -- TODO: foreign key?
  entity_index INTEGER NOT NULL,
  start_block  INTEGER NOT NULL,
  end_block    INTEGER NOT NULL,

  results_height          INTEGER NOT NULL,
  have_results            BOOLEAN NOT NULL,
  final_results           BOOLEAN NOT NULL,
  results_votes           TEXT NOT NULL DEFAULT '',
  results_weight          TEXT NOT NULL DEFAULT '',
  results_envelope_height INTEGER NOT NULL DEFAULT 0,
  results_signatures      TEXT NOT NULL DEFAULT '',
  results_block_height    INTEGER NOT NULL DEFAULT 0,

  census_root         BLOB NOT NULL,
  rolling_census_root BLOB NOT NULL,
  rolling_census_size INTEGER NOT NULL,
  max_census_size     INTEGER NOT NULL,
  census_uri          TEXT NOT NULL,
  metadata            TEXT NOT NULL,
  census_origin       INTEGER NOT NULL,
  status              INTEGER NOT NULL,
  namespace           INTEGER NOT NULL,

  envelope_pb  BLOB NOT NULL,
  mode_pb      BLOB NOT NULL,
  vote_opts_pb BLOB NOT NULL,

  private_keys TEXT NOT NULL, -- comma-separated list of hex keys
  public_keys  TEXT NOT NULL, -- comma-separated list of hex keys

  question_index      INTEGER NOT NULL,
  creation_time       DATETIME NOT NULL,
  source_block_height INTEGER NOT NULL,
  source_network_id   INTEGER NOT NULL
);

CREATE INDEX index_processes_entity_id
ON processes(entity_id);

CREATE INDEX index_processes_namespace
ON processes(namespace);

-- +goose Down
DROP TABLE processes

DROP INDEX index_processes_entity_id

DROP INDEX index_processes_namespace
