# eps-go
## A simple implementation of an electrum personal server
====================

### Motivation

I was running the electrum personal server implementation by Chris Belcher, but
it is not maintained anymore, does not support concurrent connections and 
depends on legacy wallet (removed on bitcoin core v30) to work. So I was forced
to create something new.

This implementation allows you to track xpubs for N wallets.

### Limitations
* The current implementation does not track mempool transactions

### Runtime Dependencies
* A trusted bitcoin node.

### Hardware requirements (as of Jan 2026)
* Memory: 100MB
* Disk: 200MB

### Instructions

* Build
```
git clone https://github.com/ncodysoftware/eps-go
cd eps-go
make
```

* Configure the server copying `~/.config/eps-go/eps-go.conf.example` to
`~/.config/eps-go/eps-go.conf` and setting the config parameters / adding 
`WALLET_` entries as you need.

* Run
```
./out/eps-go
```

### Tips
* To speed up the first synchronization, mount a ramfs and point the sqlite 
database to the ramfs. After the synchronization, copy the database file back to
the persistent storage.
