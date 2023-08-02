-- +goose Up
CREATE TABLE processes (
  id           BLOB NOT NULL PRIMARY KEY,
  entity_id    BLOB NOT NULL,
  start_block  INTEGER NOT NULL,
  end_block    INTEGER NOT NULL,
  block_count  INTEGER NOT NULL,

  have_results            BOOLEAN NOT NULL,
  final_results           BOOLEAN NOT NULL,
  results_votes           TEXT NOT NULL, -- json integer-string matrix, e.g. [["3", "4"]]
  results_weight          TEXT NOT NULL, -- json integer-string, e.g. "3"
  results_block_height    INTEGER NOT NULL,

  census_root         BLOB NOT NULL,
  rolling_census_root BLOB NOT NULL,
  rolling_census_size INTEGER NOT NULL,
  max_census_size     INTEGER NOT NULL,
  census_uri          TEXT NOT NULL,
  metadata            TEXT NOT NULL,
  census_origin       INTEGER NOT NULL,
  status              INTEGER NOT NULL,
  namespace           INTEGER NOT NULL,

  envelope  TEXT NOT NULL, -- json object
  mode      TEXT NOT NULL, -- json object
  vote_opts TEXT NOT NULL, -- json object

  private_keys TEXT NOT NULL, -- json array
  public_keys  TEXT NOT NULL, -- json array

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
