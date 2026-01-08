---
BEGIN;
---
CREATE TABLE IF NOT EXISTS spent_output (
	txid_vout BLOB PRIMARY KEY,
	satoshi INTEGER NOT NULL,
	scriptpubkey_hash BLOB NOT NULL,
	spent_height INTEGER NOT NULL
);
---
COMMIT;
---
