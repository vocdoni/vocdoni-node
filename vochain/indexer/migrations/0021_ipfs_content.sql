-- +goose Up
-- Local cache of IPFS content fetched by the importmetadata tool. The primary
-- key is the bare CID (without the ipfs:// prefix). Rows are inserted by the
-- offline importer and read by the API to inline metadata without any network
-- call. The table intentionally stores raw bytes so the API can forward them
-- without re-encoding.
CREATE TABLE ipfs_content (
	cid  TEXT PRIMARY KEY,
	data BLOB NOT NULL
);

-- +goose Down
DROP TABLE ipfs_content;
