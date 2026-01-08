#!/bin/env bash
####################
set -e
####################
readonly RELDIR="$(dirname ${0})"
readonly HELP_MSG='usage: < help >'
####################
eprintln()
{
	! [ -z "${1}" ] || eprintln "eprintln: missing message"
	printf "${1}\n" 1>&2
	exit 1
}
dist()
{
	rm -rf out/eps-go-${1}-${2}
	mkdir -p out/eps-go-${1}-${2}
	CGO_ENABLED=0 GOOS=${1} GOARCH=${2} go build \
	-o ./out/eps-go-${1}-${2} \
	-ldflags='-s -w' \
	./cmd/eps-go
	tar -czf out/eps-go-${1}-${2}.tar.gz out/eps-go-${1}-${2}
}
dist_all()
{
	dist darwin amd64
	dist darwin arm64
	dist linux amd64
	dist linux arm64
	dist windows amd64
	dist windows arm64
}
####################
case "${1}" in
	all) dist_all ;;
	*) eprintln "${HELP_MSG}" ;;
esac
