---
BEGIN;
---
CREATE TABLE IF NOT EXISTS blockheader (
	height INTEGER PRIMARY KEY,
	data BLOB NOT NULL
);
---
END;
