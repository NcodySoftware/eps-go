package walletsync

import (
	"context"

	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

type blockData struct {
	Hash [32]byte
	Height int 
}

func rSelectLastBlockHeaderData(
	ctx context.Context,
	db sql.Database,
	out *blockData,
) error {
	s := `
	SELECT hash, height
	FROM blockheader
	ORDER BY height DESC
	LIMIT 1
	;
	`
	row := db.QueryRow(ctx, s)
	tmp := make([]byte, 0, 32)
	err := row.Scan(&tmp, &out.Height)
	if err != nil {
		return stackerr.Wrap(err)
	}
	copy(out.Hash[:], tmp[:32])
	return nil
}

func rInsertBlockHeader(
	ctx context.Context,
	db sql.Database,
	hash *[32]byte,
	height int,
	serialized []byte,
) error {
	s := `
	INSERT INTO blockheader
	(hash, height, serialized)
	SELECT $1, $2, $3
	WHERE
		NOT EXISTS (
			SELECT 1
			FROM blockheader
			WHERE
				hash = $1
		)
	;
	`
	_, err := db.Exec(ctx, s, hash[:], height, serialized)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}
