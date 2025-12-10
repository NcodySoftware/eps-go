package walletsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"slices"
	"testing"

	"github.com/ncodysoftware/eps-go/testutil"
	"ncody.com/ncgo.git/assert"
	"ncody.com/ncgo.git/bitcoin"
	"ncody.com/ncgo.git/bitcoin/bip32"
	"ncody.com/ncgo.git/bitcoin/scriptpubkey"
	"ncody.com/ncgo.git/database/sql"
	"ncody.com/ncgo.git/stackerr"
)

func TestIntegration_walletmanager(t *testing.T) {
	tc, cls := testutil.GetTCtx(t)
	defer cls()
	seed := testutil.MustHexDecode(
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	)
	pub, _, err := bip32.DeriveSeed(seed, []uint32{0 | bip32.KEY_HARDENED})
	wParams := walletParams{
		CreatedAtHeight: 0,
		ScriptKind:      scriptpubkey.SK_P2WPKH,
		Reqsigs:         1,
		KeySet:          []bip32.ExtendedKey{pub},
	}
	wm, err := NewWalletManager(tc.C, tc.D, wParams, tc.L)
	assert.Must(t, err)
	height := 1
	blockHash := [32]byte{0: 1}
	// txs
	sp := mustScriptPubkey(t, &wParams, 0, 0)
	sp2 := mustScriptPubkey(t, &wParams, 1, 0)
	sp3 := mustScriptPubkey(t, &wParams, 3, 0)
	_, ok := wm.scriptPubkeys[sha256.Sum256(sp)]
	assert.MustEqual(t, true, ok)
	_, ok = wm.scriptPubkeys[sha256.Sum256(sp2)]
	assert.MustEqual(t, true, ok)
	_, ok = wm.scriptPubkeys[sha256.Sum256(sp3)]
	assert.MustEqual(t, false, ok)
	var txs [2]bitcoin.Transaction
	txs[0] = bitcoin.Transaction{
		Version:     0,
		Marker:      0,
		Flag:        0,
		InputCount:  0,
		Inputs:      []bitcoin.Input{},
		OutputCount: 1,
		Outputs: []bitcoin.Output{
			{
				Amount:           1_000_000,
				ScriptPubKeySize: uint64(len(sp)),
				ScriptPubkey:     sp,
			},
		},
	}
	txs[1] = bitcoin.Transaction{
		Version:    0,
		Marker:     0,
		Flag:       0,
		InputCount: 0,
		Inputs: []bitcoin.Input{
			{
				Txid: txs[0].Txid(nil),
				Vout: 0,
			},
		},
		OutputCount: 1,
		Outputs: []bitcoin.Output{
			{
				Amount:           500_000,
				ScriptPubKeySize: uint64(len(sp2)),
				ScriptPubkey:     sp2,
			},
			{
				Amount:           500_000,
				ScriptPubKeySize: uint64(len(sp3)),
				ScriptPubkey:     sp3,
			},
		},
	}
	txid := txs[1].Txid(nil)
	var txidVout [32 + 4]byte
	makeTxidVout(&txid, 0, txidVout[:0])
	expUtxoSet := []utxoData{
		{
			TxidVout:         txidVout,
			Satoshi:          500_000,
			ScriptPubkeyHash: sha256.Sum256(sp2),
		},
	}
	expScriptPubkeyTxSet := []scriptPubkeyTxData{
		{
			ScriptPubkeyHash: sha256.Sum256(sp),
			Txid:             txs[0].Txid(nil),
		},
		{
			ScriptPubkeyHash: sha256.Sum256(sp),
			Txid:             txs[1].Txid(nil),
		},
		{
			ScriptPubkeyHash: sha256.Sum256(sp2),
			Txid:             txs[1].Txid(nil),
		},
	}
	expAccountSet := []accountData{
		{
			Hash:      wm.accounts[0].Hash,
			NextIndex: 1,
			Height:    height,
		},
		{
			Hash:      wm.accounts[1].Hash,
			NextIndex: 1,
			Height:    height,
		},
	}
	walletManagerTest(
		t,
		tc,
		wm,
		height,
		&blockHash,
		txs[:],
		expUtxoSet,
		expScriptPubkeyTxSet,
		expAccountSet,
	)
}

func walletManagerTest(
	t *testing.T,
	tc *testutil.TCtx,
	wm *walletManager,
	height int,
	blockHash *[32]byte,
	txs []bitcoin.Transaction,
	expUtxoSet []utxoData,
	expScriptPubkeyTxSet []scriptPubkeyTxData,
	expAccountSet []accountData,
) {
	ctx := tc.C
	db := tc.D
	for _, tx := range txs {
		txid := tx.Txid(nil)
		err := wm.HandleTransaction(
			ctx,
			db,
			height,
			blockHash,
			&txid,
			&tx,
		)
		assert.Must(t, err)
	}
	sortUtxoData(expUtxoSet)
	sortScriptPubkeyTxData(expScriptPubkeyTxSet)
	sortAccountData(expAccountSet)
	utxos, err := trSelectUtxos(tc.C, tc.D, len(expUtxoSet)+1, 0)
	assert.Must(t, err)
	shTxs, err := trSelectScriptPubkeyTxs(
		tc.C, tc.D, len(expScriptPubkeyTxSet)+1, 0,
	)
	assert.Must(t, err)
	accounts, err := trSelectAccounts(tc.C, tc.D, len(expAccountSet)+1, 0)
	assert.Must(t, err)
	assert.MustEqual(t, expUtxoSet, utxos)
	assert.MustEqual(t, expScriptPubkeyTxSet, shTxs)
	assert.MustEqual(t, expAccountSet, accounts)
}

func trSelectUtxos(
	ctx context.Context, db sql.Database, limit, offset int,
) ([]utxoData, error) {
	s := `
	SELECT txid_vout, satoshi, scriptpubkey_hash
	FROM unspent_output
	ORDER BY txid_vout ASC
	LIMIT $1
	OFFSET $2
	;
	`
	rows, err := db.Query(ctx, s, limit, offset)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	defer rows.Close()
	var r []utxoData
	for rows.Next() {
		var u utxoData
		txv := bufWrapper(u.TxidVout[:])
		sh := bufWrapper(u.ScriptPubkeyHash[:])
		err := rows.Scan(&txv, &u.Satoshi, &sh)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		r = append(r, u)
	}
	return r, nil
}

func trSelectScriptPubkeyTxs(
	ctx context.Context, db sql.Database, limit, offset int,
) ([]scriptPubkeyTxData, error) {
	s := `
	SELECT scriptpubkey_hash, txid
	FROM scriptpubkey_tx
	ORDER BY scriptpubkey_hash ASC
	LIMIT $1
	OFFSET $2
	;
	`
	rows, err := db.Query(ctx, s, limit, offset)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	defer rows.Close()
	var r []scriptPubkeyTxData
	for rows.Next() {
		var s scriptPubkeyTxData
		sh := bufWrapper(s.ScriptPubkeyHash[:])
		txid := bufWrapper(s.Txid[:])
		err := rows.Scan(&sh, &txid)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		r = append(r, s)
	}
	return r, nil
}

func trSelectAccounts(
	ctx context.Context, db sql.Database, limit, offset int,
) ([]accountData, error) {
	s := `
	SELECT hash, next_index, height
	FROM account
	ORDER BY hash ASC
	LIMIT $1
	OFFSET $2
	;
	`
	rows, err := db.Query(ctx, s, limit, offset)
	if err != nil {
		return nil, stackerr.Wrap(err)
	}
	defer rows.Close()
	var r []accountData
	for rows.Next() {
		var a accountData
		hash := bufWrapper(a.Hash[:])
		err := rows.Scan(&hash, &a.NextIndex, &a.Height)
		if err != nil {
			return nil, stackerr.Wrap(err)
		}
		r = append(r, a)
	}
	return r, nil
}

func sortUtxoData(u []utxoData) {
	slices.SortFunc(u, func(a, b utxoData) int {
		return bytes.Compare(a.TxidVout[:], b.TxidVout[:])
	})
}

func sortScriptPubkeyTxData(s []scriptPubkeyTxData) {
	slices.SortFunc(s, func(a, b scriptPubkeyTxData) int {
		return bytes.Compare(
			a.ScriptPubkeyHash[:], b.ScriptPubkeyHash[:],
		)
	})
}

func sortAccountData(s []accountData) {
	slices.SortFunc(s, func(a, b accountData) int {
		return bytes.Compare(a.Hash[:], b.Hash[:])
	})
}

func mustScriptPubkey(
	t *testing.T, m *walletParams, account, index uint32,
) []byte {
	accountKeys := make([]*bip32.ExtendedKey, 0, len(m.KeySet))
	path := [1]uint32{account}
	for _, k := range m.KeySet {
		acc, err := bip32.DeriveXpub(&k, path[:])
		assert.Must(t, err)
		accountKeys = append(accountKeys, &acc)
	}
	spks, err := scriptpubkey.MakeMulti(
		m.ScriptKind, m.Reqsigs, index, 1, accountKeys,
	)
	assert.Must(t, err)
	return spks[0]
}
