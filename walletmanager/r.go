package walletmanager

func rSelectBlockHeight(
	ctx ct, db dt, hash [32]byte,
) (int, error) {
	s := `
	SELECT (height)
	FROM blockheader
	WHERE
		hash = $1
	;
	`
	var height int
	row := db.QueryRow(ctx, s, hash[:])
	err := row.Scan(&height)
	if err != nil {
		return 0, stackerr.Wrap(err)
	}
	return height, nil
}

func rInsertBlockHeader(
	ctx ct,
	db dt,
	hash [32]byte,
	height int,
	serialized []byte,
) error {
    s := `
    INSERT INTO blockheader
    (hash, height, serialized)
    VALUES
    ($1, $2, $3)
    ;
    `
    _, err := db.Exec(ctx, s, hash[:], height, serialized)
    if err != nil {
	    return stackerr.Wrap(err)
    }
    return nil
}

func rInsertTransaction(
	ctx ct, db dt, txid [32]byte, blockhash [32]byte, serializedTx []byte,
) error {
	s := `
	INSERT INTO transaction
	(txid, blockhash, serialized)
	SELECT $1, $2, $3
	WHERE
		NOT EXISTS (
			SELECT 1 
			FROM transaction
			WHERE
				txid = $1
		)
	;
	`
	_, err := db.Exec(ctx, s, txid[:], blockhash[:], serializedTx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}
