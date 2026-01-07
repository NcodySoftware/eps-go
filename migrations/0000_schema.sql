---
BEGIN;
---
CREATE TABLE IF NOT EXISTS wallet (
	hash BLOB PRIMARY KEY,
	height INTEGER NOT NULL,
	next_receive_index INTEGER NOT NULL,
	next_change_index INTEGER NOT NULL
);
---
CREATE TABLE IF NOT EXISTS blockheader (
	hash BLOB PRIMARY KEY,
	height INTEGER NOT NULL,
	serialized BLOB NOT NULL
);
CREATE INDEX blockheader_height_idx
ON blockheader (height);
---
--CREATE TABLE IF NOT EXISTS block (
--	hash BLOB PRIMARY KEY,
--	height INTEGER NOT NULL,
--	serialized BLOB NOT NULL
--);
---
CREATE TABLE IF NOT EXISTS unspent_output (
	txid_vout BLOB PRIMARY KEY,
	satoshi INTEGER NOT NULL,
	scriptpubkey_hash BLOB NOT NULL
);
---
CREATE TABLE IF NOT EXISTS tx (
	txid BLOB PRIMARY KEY,
	blockhash BLOB NOT NULL,
	pos INTEGER NOT NULL,
	serialized BLOB NOT NULL,
	merkle_proof BLOB NOT NULL
);
---
CREATE TABLE IF NOT EXISTS scriptpubkey_tx (
	scriptpubkey_hash BLOB NOT NULL,
	txid BLOB NOT NULL
);
---
COMMIT;
---
