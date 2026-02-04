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
	echo "building eps-go-${1}-${2}"
	rm -rf out/eps-go-${1}-${2}
	mkdir -p out/eps-go-${1}-${2}
	CGO_ENABLED=1 \
	GOOS=${1} \
	GOARCH=${2} \
	CC="${3}" \
	go build \
	-v \
	-o ./out/eps-go-${1}-${2} \
	-ldflags='-s -w' \
	./cmd/eps-go
	tar -czf out/eps-go-${1}-${2}.tar.gz out/eps-go-${1}-${2}
}
dist_all()
{
	#dist darwin	amd64	'zig cc --target=x86_64-macos'
	#dist darwin	arm64	'zig cc --target=aarch64-macos'
	dist linux	amd64	'zig cc --target=x86_64-linux'
	dist linux	386	'zig cc --target=x86-linux'
	dist linux	arm64	'zig cc --target=aarch64-linux'
	dist windows	amd64	'zig cc --target=x86_64-windows'
	dist windows	386	'zig cc --target=x86-windows'
	dist windows	arm64	'zig cc --target=aarch64-windows'
}
####################
case "${1}" in
	all) dist_all ;;
	*) eprintln "${HELP_MSG}" ;;
esac
