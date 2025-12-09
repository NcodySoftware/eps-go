---
BEGIN;
---
CREATE TABLE IF NOT EXISTS account (
	hash BLOB PRIMARY KEY,
	next_index INTEGER NOT NULL,
	height INTEGER NOT NULL
);
---
CREATE TABLE IF NOT EXISTS blockheader (
	hash BLOB PRIMARY KEY,
	height INTEGER NOT NULL,
	serialized BLOB NOT NULL
);
---
CREATE TABLE IF NOT EXISTS unspent_output (
	txid_vout BLOB PRIMARY KEY,
	satoshi INTEGER NOT NULL,
	scriptpubkey BLOB NOT NULL
);
---
--CREATE TABLE IF NOT EXISTS transaction (
--	txid BLOB PRIMARY KEY,
--	blockhash BLOB NOT NULL,
--	serialized BLOB NOT NULL
--);
---
CREATE TABLE IF NOT EXISTS scriptpubkey_transaction (
	scriptpubkey_hash BLOB NOT NULL,
	txid BLOB NOT NULL
);
---
COMMIT;
---
