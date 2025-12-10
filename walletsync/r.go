package walletsync

import (
	"context"
	"fmt"

	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

type blockHeaderData struct {
	Hash   [32]byte
	Height int
}

func rSelectLastBlockHeaderData(
	ctx context.Context,
	db sql.Database,
	out *blockHeaderData,
) error {
	s := `
	SELECT hash, height
	FROM blockheader
	ORDER BY height DESC
	LIMIT 1
	;
	`
	hs := bufWrapper(out.Hash[:])
	err := db.QueryRow(ctx, s).Scan(&hs, &out.Height)
	if err != nil {
		return stackerr.Wrap(err)
	}
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

type accountData struct {
	Hash      [32]byte
	NextIndex int
	Height    int
}

func rSelectAccountData(
	ctx context.Context,
	db sql.Database,
	hash *[32]byte,
	out *accountData,
) error {
	s := `
	SELECT next_index, height
	FROM account
	WHERE 
		hash = $1
	;
	`
	out.Hash = *hash
	row := db.QueryRow(ctx, s, hash[:])
	err := row.Scan(&out.NextIndex, &out.Height)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func rInsertAccountData(
	ctx context.Context,
	db sql.Database,
	hash *[32]byte,
	height int,
) error {
	s := `
	INSERT INTO account
	(hash, next_index, height)
	VALUES
	($1, 0, $2)
	;
	`
	_, err := db.Exec(ctx, s, hash[:], height)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func rInsertTransaction(
	ctx context.Context,
	db sql.Database,
	txid *[32]byte,
	blockHash *[32]byte,
	serializedTx []byte,
) error {
	s := `
	INSERT INTO tx
	(txid, blockhash, serialized)
	SELECT $1, $2, $3
	WHERE NOT EXISTS (
		SELECT 1 FROM tx
		WHERE txid = $1
	)
	;
	`
	_, err := db.Exec(ctx, s, txid[:], blockHash[:], serializedTx)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func rInsertUtxo(
	ctx context.Context,
	db sql.Database,
	txidVout *[32 + 4]byte,
	satoshi uint64,
	scriptPubkeyHash *[32]byte,
) error {
	s := `
	INSERT INTO unspent_output
	(txid_vout, satoshi, scriptpubkey_hash)
	SELECT $1, $2, $3
	WHERE NOT EXISTS (
		SELECT 1 FROM unspent_output
		WHERE txid_vout = $1
	)
	;
	`
	_, err := db.Exec(ctx, s, txidVout[:], satoshi, scriptPubkeyHash[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

type utxoData struct {
	TxidVout         [32 + 4]byte
	Satoshi          uint64
	ScriptPubkeyHash [32]byte
}

func rSelectAllUtxos(
	ctx context.Context,
	db sql.Database,
	out *[]utxoData,
) error {
	s := `
	SELECT txid_vout, satoshi, scriptpubkey_hash
	FROM unspent_output
	;
	`
	rows, err := db.Query(ctx, s)
	if err != nil {
		return stackerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var ud utxoData
		tv := bufWrapper(ud.TxidVout[:])
		sh := bufWrapper(ud.ScriptPubkeyHash[:])
		err := rows.Scan(&tv, &ud.Satoshi, &sh)
		if err != nil {
			return stackerr.Wrap(err)
		}
		*out = append(*out, ud)
	}
	return nil
}

func rDeleteUtxo(
	ctx context.Context,
	db sql.Database,
	txidVout *[32 + 4]byte,
) error {
	s := `
	DELETE FROM unspent_output
	WHERE txid_vout = $1
	;
	`
	_, err := db.Exec(ctx, s, txidVout[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func rInsertScriptPubkeyTx(
	ctx context.Context,
	db sql.Database,
	sHash *[32]byte,
	txid *[32]byte,
) error {
	s := `
	INSERT INTO scriptpubkey_tx
	(scriptpubkey_hash, txid)
	SELECT $1, $2
	WHERE NOT EXISTS (
		SELECT 1 FROM scriptpubkey_tx
		WHERE 
			scriptpubkey_hash = $1
		AND
			txid = $2
	)
	;
	`
	_, err := db.Exec(ctx, s, sHash[:], txid[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

type scriptPubkeyTxData struct {
	ScriptPubkeyHash [32]byte
	Txid             [32]byte
}

func rUpdateAccount(
	ctx context.Context,
	db sql.Database,
	hash *[32]byte,
	nextIndex int,
	height int,
) error {
	s := `
	UPDATE account
	SET next_index = $2, height = $3
	WHERE hash = $1
	;
	`
	_, err := db.Exec(ctx, s, hash[:], nextIndex, height)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

type bufWrapper []byte

func (self *bufWrapper) Scan(src any) error {
	data, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("bufWrapper: expecting []byte, got %T", src)
	}
	if len(data) != len(*self) {
		return fmt.Errorf("bufWrapper: len of src != len dest")
	}
	copy(*self, data)
	return nil
}
