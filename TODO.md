# CURRENT

* bip32

# ALL

* base58 #OK

* bip32 #OK

* scriptPubkey manager #OK

* blockprocessor

* mempoolprocessor

# Description

block_scan: block scriptpubkeys utxos 
	transactions 

mempool_scan: mempool scriptpubkeys utxos
	mempool_transactions 

scripthash_transactions: scripthashes transactions mempool_transactions
	transactions

# Datasets

master_key
    hash
    next_index

blockheader
    hash: primary key
    height: index
    serialized

unspent_output
    txid_vout: primary key
    satoshi
    scriptpubkey: index

transaction
    txid: primary key
    blockhash: fk
    serialized

scriptpubkey_transaction
    scriptpubkey_hash: index
    txid: fk
