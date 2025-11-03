#!/usr/bin/env sh
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
	local goos="${1}"
	local goarch="${2}"
	local cdir="${PWD}"

	! [ -z "${EPS_GO_VERSION}" ] || eprintln 'undefined EPS_GO_VERSION'

	mkdir -p ${RELDIR}/../out/eps-go-${goos}-${goarch}
	GGO_ENABLED=0 GOOS=${goos} GOARCH=${goarch} go build \
	-ldflags "-s -w -X main.Version=${EPS_GO_VERSION}" \
	-o out/eps-go-${goos}-${goarch}/eps-go ${RELDIR}/../

	cd ${RELDIR}/../out
	tar -czf eps-go-${goos}-${goarch}.tar.gz eps-go-${goos}-${goarch}
	cd "${cdir}"
}
dist_all()
{
	dist linux amd64
	dist linux arm64
	dist darwin amd64
	dist darwin arm64
	dist windows amd64
	dist windows arm64
}
####################
dist_all
