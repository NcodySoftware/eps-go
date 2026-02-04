# v0.2.1 - Huge performance boost

* Bugfix: allow subscription to scripthashes with null status.
* Bugfix: rescan caused by wallet addition was generating conflicts on
spent_output table, added check that only inserts the entry when txid_vout
does not exist in the table.
* Bugfix: Electrum protocol server.version now works with params being in the
formats [clientVersion,protocolVersion] and 
[clientVersion,[protocolVersionMin,protocolVersionMax]]
* Bitcoin Client: from crash to auto reconnect after network failure
* Reduced module dependency graph from 49 modules to 1 with internalization of
vendor modules.
* Migrated database backend from modernc.org/sqlite to
github.com/mattn/go-sqlite3. Using CGO, but can be cross compiled with `zig cc` 

# Benchmark:
* Time to sync 10000 regtest blocks containing a single coinbase transaction
to the first receiving address of the wallet being tracked.
* CPU: Core I5-3373U (HP Laptop from 2012)
* RAM: 8GB DDR3

## Using modernc.org/sqlite (OLD)
* run1 (ramfs): 10m32s
* run2 (ramfs after optimization): 38s
* run3 (on disk): 15m45s
* run4 (on disk): 15m35s

## Using github.com/mattn/go-sqlite3 (NEW)
* run1 (ramfs): 19m52s
* run2 (ramfs): 21m
* run3 (ramfs after optimization): 23s
* run4 (on disk): 4m3s
* run5 (on disk): 3m25s

Don't query scripthash transactions if there are no electrum subscribers. A 
single `if` before the query delivered this huge speed improvement.
