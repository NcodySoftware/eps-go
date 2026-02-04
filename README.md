# eps-go - Lightweight Electrum Personal Server
## Connect Electrum compatible wallets with your Bitcoin Node
===================

### Motivation

I was running the electrum personal server implementation by Chris Belcher, but
it is not maintained anymore, does not support concurrent connections and 
depends on legacy wallet (removed on bitcoin core v30) to work. So I was forced
to create something new.

### Features
* Just a single binary
* Low resource usage
* Track multiple wallets: p2pk, p2pkh, p2ms, p2sh, p2sh_wpkh, p2wpkh, p2wsh 
* No TX index needed
* Your Bitcoin node can be pruned
* Concurrent connections are supported

### Limitations
* For now, there's no mempool

### Build Dependencies
* Go compiler
* C compiler: if you want static executables, `zig cc`, which wraps clang
is a good tool

### Runtime Dependencies
* A trusted bitcoin node (any implementation that provides headers and blocks
via the P2P protocol)

### Hardware requirements
* Memory: 100MB
* Disk: 200MB

### Configuration
* Configure the server copying `~/.config/eps-go/eps-go.conf.example` to
`~/.config/eps-go/eps-go.conf` and setting the config parameters, adding 
`WALLET_` entries as you need, as the example below:
```
WALLET_segwit_native = p2wpkh xpub...
WALLET_segwit_multisig = p2wsh 2 xpub1 xpub2 xpub3

```

### Building
```
git clone https://github.com/ncodysoftware/eps-go
cd eps-go
make
```

### Run
```
./out/eps-go
```

### Tips
* To speed up the first synchronization, mount a ramfs and point the sqlite 
database to the ramfs. After the synchronization, copy the database file back to
the persistent storage.
