BTC_NETWORK := regtest
LOG_LEVEL := WARN
BTC_NODE_ADDR := 127.0.0.1:18444
SQLITE_DB_PATH := $(PWD)/tmp/db.sqlite3

.PHONY: all FORCE fmt ctags testint run dist

all: out/eps-go

out/eps-go: FORCE
	@mkdir -p out
	go build -v -o out/eps-go -ldflags='-s -w' ./cmd/eps-go

fmt:
	@find . -type f -name '*.go' -not -path './vendor/*' \
	-exec gofmt -w {} \;

testint:
	mkdir -p $(PWD)/tmp
	BTC_NETWORK=$(BTC_NETWORK) \
	LOG_LEVEL=$(LOG_LEVEL) \
	BTC_NODE_ADDR=$(BTC_NODE_ADDR) \
	SQLITE_DB_PATH=$(SQLITE_DB_PATH) \
	go test ./... -count=1 -p=1 -v 

FORCE:	

dist:
	./scripts/dist.sh all
