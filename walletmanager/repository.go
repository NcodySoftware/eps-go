package walletmanager

import (
	"context"
	"errors"
	"fmt"

	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

var errNotFound = errors.New("not found")

type utxoData2 struct {
	Satoshi          uint64
	ScriptPubkeyHash [32]byte
}

type repository struct {
	db        sql.Database
	utxoIndex map[txidVout]utxoData2
}

func newRepository(ctx context.Context, db sql.Database) (*repository, error) {
	r := &repository{
		db: db,
	}
	err := r.loadUtxoIndex(ctx, db)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return r, nil
}

func (r *repository) loadUtxoIndex(ctx context.Context, db sql.Database) error {
	utxos, err := r.selectAllUtxos(ctx, db)
	if err != nil {
		return stackerr.Wrap(err)
	}
	if r.utxoIndex == nil {
		r.utxoIndex = make(map[txidVout]utxoData2, len(utxos))
	}
	for i := range utxos {
		r.utxoIndex[utxos[i].TxidVout] = utxoData2{
			Satoshi:          utxos[i].Satoshi,
			ScriptPubkeyHash: utxos[i].ScriptPubkeyHash,
		}
	}
	return nil
}

func (r *repository) reloadCaches(
	ctx context.Context,
	db sql.Database,
) error {
	for k := range r.utxoIndex {
		delete(r.utxoIndex, k)
	}
	err := r.loadUtxoIndex(ctx, db)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

type walletData struct {
	Hash             [32]byte
	Height           int
	NextReceiveIndex uint32
	NextChangeIndex  uint32
}

func (r *repository) selectWalletData(
	ctx context.Context,
	db sql.Database,
	hash *[32]byte,
) (walletData, error) {
	s := `
	SELECT height, next_receive_index, next_change_index
	FROM wallet
	WHERE hash = $1
	LIMIT 1;
	`
	var w walletData
	err := db.QueryRow(ctx, s, hash[:]).Scan(
		&w.Height, &w.NextReceiveIndex, &w.NextChangeIndex,
	)
	if err != nil {
		return w, stackerr.Wrap(err)
	}
	w.Hash = *hash
	return w, nil
}

func (r *repository) insertWalletData(
	ctx context.Context,
	db sql.Database,
	wd *walletData,
) error {
	s := `
	INSERT INTO wallet 
	(hash, height, next_receive_index, next_change_index)
	VALUES
	($1, $2, $3, $4)
	;
	`
	_, err := db.Exec(
		ctx,
		s,
		wd.Hash[:],
		wd.Height,
		wd.NextReceiveIndex,
		wd.NextChangeIndex,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

type blockHeaderData struct {
	Hash       [32]byte
	Height     int
	Serialized []byte
}

func (r *repository) selectLastBlockHeaderData(
	ctx context.Context,
	db sql.Database,
	hd *blockHeaderData,
) error {
	s := `
	SELECT hash, height, serialized
	FROM blockheader
	ORDER BY height DESC
	LIMIT 1
	;
	`
	h := bufWrapper(hd.Hash[:])
	err := db.QueryRow(ctx, s).Scan(&h, &hd.Height, &hd.Serialized)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (r *repository) selectBlockHashesAtHeight(
	ctx context.Context,
	db sql.Database,
	height int,
	limit int,
	out *[][32]byte,
) error {
	s := `
	SELECT hash FROM blockheader
	WHERE
		EXISTS (
			SELECT 1 FROM blockheader
			WHERE height = $1
		)
		AND
		height >= $1
	ORDER BY height ASC
	LIMIT $2
	;
	`
	rows, err := db.Query(ctx, s, height, limit)
	if err != nil {
		return stackerr.Wrap(err)
	}
	defer rows.Close()
	for i := 0; rows.Next(); i++ {
		var buf [32]byte
		h := bufWrapper(buf[:])
		err := rows.Scan(&h)
		if err != nil {
			return stackerr.Wrap(err)
		}
		*out = append(*out, buf)
	}
	return nil
}

func (r *repository) selectRawBlockHeaderByHeight(
	ctx context.Context,
	db sql.Database,
	height int,
	out *[80]byte,
) error {
	s := `
	SELECT (serialized)
	FROM blockheader
	WHERE height = $1
	LIMIT 1;
	`
	bw := bufWrapper(out[:])
	err := db.QueryRow(ctx, s, height).Scan(&bw)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (r *repository) selectRawBlockHeadersByHeight(
	ctx context.Context,
	db sql.Database,
	height,
	limit int,
) ([][80]byte, error) {
	s := `
	SELECT (serialized)
	FROM blockheader
	WHERE height >= $1
	ORDER BY height ASC
	LIMIT $2
	;
	`
	rows, err := db.Query(ctx, s, height, limit)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	defer rows.Close()
	var h [][80]byte
	for rows.Next() {
		var b [80]byte
		bw := bufWrapper(b[:])
		err := rows.Scan(&bw)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		h = append(h, b)
	}
	return h, nil
}

func (r *repository) selectLastBlockHeaderHeightAndRaw(
	ctx context.Context,
	db sql.Database,
	outHeight *int,
	outHeader *[80]byte,
) error {
	s := `
	SELECT height, serialized
	FROM blockheader
	ORDER BY height DESC
	LIMIT 1
	;
	`
	h := bufWrapper(outHeader[:])
	err := db.QueryRow(ctx, s).Scan(outHeight, &h)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (r *repository) selectScriptHashBalance(
	ctx context.Context,
	db sql.Database,
	sh *[32]byte,
	out *uint64,
) error {
	_, _ = ctx, db
	*out = 0
	for _, v := range r.utxoIndex {
		if v.ScriptPubkeyHash != *sh {
			continue
		}
		*out += v.Satoshi
	}
	return nil
}

type TxData struct {
	Height int
	Txid   [32]byte
}

func (r *repository) selectScriptHashHistory(
	ctx context.Context,
	db sql.Database,
	sh *[32]byte,
) ([]TxData, error) {
	s := `
	SELECT bh.height, stx.txid
	FROM scriptpubkey_tx AS stx
	JOIN tx 
	ON tx.txid = stx.txid
	JOIN blockheader AS bh 
	ON bh.hash = tx.blockhash
	WHERE stx.scriptpubkey_hash = $1
	ORDER BY bh.height, tx.pos ASC
	;
	`
	var h []TxData
	rows, err := db.Query(ctx, s, sh[:])
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var swap TxData
		bw := bufWrapper(swap.Txid[:])
		err := rows.Scan(&swap.Height, &bw)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		h = append(h, swap)
	}
	return h, nil
}

type UtxoData struct {
	Height  int
	TxPos   int
	Txid    [32]byte
	Satoshi uint64
}

func (r *repository) selectScriptHashUnspent(
	ctx context.Context,
	db sql.Database,
	sh *[32]byte,
) ([]UtxoData, error) {
	s := `
	SELECT bh.height, tx.pos, SUBSTR(uo.txid_vout, 1, 32), uo.satoshi
	FROM unspent_output AS uo
	JOIN tx
	ON tx.txid = SUBSTR(uo.txid_vout, 1, 32)
	JOIN blockheader as bh
	ON bh.hash = tx.blockhash
	WHERE uo.scriptpubkey_hash = $1
	ORDER BY bh.height, tx.pos ASC
	;
	`
	var u []UtxoData
	rows, err := db.Query(ctx, s, sh[:])
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var swap UtxoData
		bw := bufWrapper(swap.Txid[:])
		err := rows.Scan(&swap.Height, &swap.TxPos, &bw, &swap.Satoshi)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		u = append(u, swap)
	}
	return u, nil
}

func (r *repository) insertBlockHeader(
	ctx context.Context,
	db sql.Database,
	hash *[32]byte,
	height int,
	serialized []byte,
) error {
	s := `
	INSERT INTO blockheader
	(hash, height, serialized)
	VALUES ($1, $2, $3)
	;
	`
	_, err := db.Exec(ctx, s, hash[:], height, serialized[:80])
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (r *repository) insertTransaction(
	ctx context.Context,
	db sql.Database,
	txid *[32]byte,
	blockHash *[32]byte,
	pos int,
	serialized []byte,
	merkleProof []byte,
) error {
	s := `
	INSERT INTO tx
	(txid, blockhash, pos, serialized, merkle_proof)
	SELECT $1, $2, $3, $4, $5
	WHERE NOT EXISTS (
		SELECT 1 FROM tx
		WHERE txid = $1
	)
	;
	`
	_, err := db.Exec(
		ctx,
		s,
		txid[:],
		blockHash[:],
		pos,
		serialized,
		merkleProof,
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (r *repository) selectRawTransaction(
	ctx context.Context,
	db sql.Database,
	txid *[32]byte,
) ([]byte, error) {
	s := `
	SELECT serialized
	FROM tx
	WHERE txid = $1
	;
	`
	var raw []byte
	err := db.QueryRow(ctx, s, txid[:]).Scan(&raw)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	return raw, nil
}

type transactionData struct {
	Txid        [32]byte
	BlockHash   [32]byte
	Pos         int
	Raw         []byte
	MerkleProof []byte
}

func (r *repository) selectTransactionFromTxid(
	ctx context.Context,
	db sql.Database,
	txid *[32]byte,
) (transactionData, error) {
	s := `
	SELECT tx.blockhash, tx.pos, tx.serialized, tx.merkle_proof
	FROM tx
	WHERE txid = $1
	LIMIT 1;
	;
	`
	var t transactionData
	bh := bufWrapper(t.BlockHash[:])
	err := db.QueryRow(ctx, s, txid[:]).Scan(
		&bh, &t.Pos, &t.Raw, &t.MerkleProof,
	)
	if err != nil {
		return t, stackerr.Wrap(err)
	}
	t.Txid = *txid
	return t, nil
}

func (r *repository) selectTransactionFromHeightPos(
	ctx context.Context,
	db sql.Database,
	height, pos int,
) (transactionData, error) {
	s := `
	SELECT tx.txid, tx.blockhash, tx.pos, tx.serialized, tx.merkle_proof
	FROM tx
	JOIN blockheader AS bh
	ON bh.hash = tx.blockhash
	WHERE
		bh.height = $1
		AND
		tx.pos = $2
	LIMIT 1;
	;
	`
	var t transactionData
	txid := bufWrapper(t.Txid[:])
	bh := bufWrapper(t.BlockHash[:])
	err := db.QueryRow(ctx, s, height, pos).Scan(
		&txid, &bh, &t.Pos, &t.Raw, &t.MerkleProof,
	)
	if err != nil {
		return t, stackerr.Wrap(err)
	}
	return t, nil
}

func (r *repository) insertScriptPubkeyTransaction(
	ctx context.Context,
	db sql.Database,
	scriptPubkeyHash *[32]byte,
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
	_, err := db.Exec(ctx, s, scriptPubkeyHash[:], txid[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (r *repository) insertUnspentOutput(
	ctx context.Context,
	db sql.Database,
	txVout *txidVout,
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
		LIMIT 1
	)
	;
	`
	_, err := db.Exec(ctx, s, txVout[:], satoshi, scriptPubkeyHash[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	r.utxoIndex[*txVout] = utxoData2{
		Satoshi:          satoshi,
		ScriptPubkeyHash: *scriptPubkeyHash,
	}
	return nil
}

func (r *repository) selectAllUtxos(
	ctx context.Context,
	db sql.Database,
) ([]utxoData, error) {
	var u []utxoData
	s := `
	SELECT txid_vout, satoshi, scriptpubkey_hash
	FROM unspent_output
	`
	rows, err := db.Query(ctx, s)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s utxoData
		tv := bufWrapper(s.TxidVout[:])
		h := bufWrapper(s.ScriptPubkeyHash[:])
		err := rows.Scan(&tv, &s.Satoshi, &h)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		u = append(u, s)
	}
	return u, nil
}

type utxoData struct {
	TxidVout         txidVout
	Satoshi          uint64
	ScriptPubkeyHash [32]byte
}

func (r *repository) selectUnspentOutput(
	ctx context.Context,
	db sql.Database,
	txVout *txidVout,
	out *utxoData,
) error {
	_, _ = ctx, db
	u, ok := r.utxoIndex[*txVout]
	if !ok {
		return errNotFound
	}
	out.TxidVout = *txVout
	out.Satoshi = u.Satoshi
	out.ScriptPubkeyHash = u.ScriptPubkeyHash
	return nil
}

func (r *repository) deleteUnspentOutput(
	ctx context.Context,
	db sql.Database,
	txVout *txidVout,
) error {
	s := `
	DELETE FROM unspent_output
	WHERE txid_vout = $1
	;
	`
	_, err := db.Exec(ctx, s, txVout[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	delete(r.utxoIndex, *txVout)
	return nil
}

func (r *repository) updateWalletIndexes(
	ctx context.Context,
	db sql.Database,
	whash *[32]byte,
	nextReceiveIndex uint32,
	nextChangeIndex uint32,
) error {
	s := `
	UPDATE wallet
	SET next_receive_index = $2, next_change_index = $3
	WHERE hash = $1
	;
	`
	_, err := db.Exec(ctx, s, whash[:], nextReceiveIndex, nextChangeIndex)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (r *repository) updateWalletHeight(
	ctx context.Context,
	db sql.Database,
	whash *[32]byte,
	height int,
) error {
	s := `
	UPDATE wallet
	SET height = $2
	WHERE hash = $1
	;
	`
	_, err := db.Exec(ctx, s, whash[:], height)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (r *repository) deleteAllSinceBlock(
	ctx context.Context,
	db sql.Database,
	height int,
) error {
	s := `
	BEGIN;
	--
	DELETE 
	FROM scriptpubkey_tx AS stx
	WHERE stx.txid IN (
		SELECT txid FROM tx
		JOIN blockheader AS bh
		ON bh.hash = tx.blockhash
		AND bh.height > $1
	);
	--
	DELETE
	FROM unspent_output
	WHERE SUBSTR(txid_vout, 1, 32) IN (
		SELECT tx.txid FROM tx
		JOIN blockheader AS bh
		ON bh.hash = tx.blockhash
		WHERE bh.height > $1
	);
	--
	DELETE
	FROM tx
	WHERE blockhash IN (
		SELECT hash
		FROM blockheader
		WHERE height > $1
	);
	--
	DELETE
	FROM blockheader
	WHERE height > $1;
	--
	UPDATE wallet
	SET height = $1
	WHERE height > $1;
	--
	COMMIT;
	`
	_, err := db.Exec(ctx, s, height)
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
