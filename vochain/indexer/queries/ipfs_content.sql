-- name: InsertIPFSContent :execresult
-- Inserts IPFS content by bare CID (without the ipfs:// prefix). OR IGNORE so
-- re-running the importer is safe: existing rows are left untouched and the
-- call succeeds.
INSERT OR IGNORE INTO ipfs_content (cid, data) VALUES (?, ?);

-- name: GetIPFSContent :one
-- Returns the raw content for a bare CID, or sql.ErrNoRows when absent.
SELECT data FROM ipfs_content WHERE cid = ?;
