# eps-go
## A simple implementation of an electrum personal server
====================

### Motivation

I was running the electrum personal server implementation by Chris Belcher, but
it is not maintained anymore, does not support concurrent connections and 
depends on legacy wallet (removed on bitcoin core v30) to work. So I was forced
to create something new.

This simple implementation allows you to import descriptors for N wallets, which
will be imported on bitcoin core.

To import wallets, you must convert the extended public keys to valid bitcoin
output descriptors, you can convert with:
```
./out/eps-go toxpub <zpub/ypub...> [mainnet|testnet]
```

### Dependencies

* make
* go compiler
* bitcoin core, which can be pruned and does not need txindex

### Instructions

* Build
```
git clone https://github.com/ncodysoftware/eps-go
cd eps-go
make
```

* Run
```
./out/eps-go
```

The first run will fail, copy `~/.config/eps-go/eps-go.conf.example` to
`~/.config/eps-go/eps-go.conf` and set the config parameters
