#!/usr/bin/env bash
####################
set -e
####################
readonly RELDIR="$(dirname ${0})"
readonly HELP_MSG='usage: < help >'
readonly EXTERNAL_DIR="${RELDIR}/../external"
readonly EXTERNAL_TMP_DIR="${RELDIR}/../tmp/external"
####################
eprintln()
{
	! [ -z "${1}" ] || eprintln "eprintln: missing message"
	printf "${1}\n" 1>&2
	exit 1
}

base58()
{
	local url="https://github.com/akamensky/base58"
	local commit="ce8bf8802e8f95a5b81482e280084dfd913167d0"
	local tmpdest="${EXTERNAL_TMP_DIR}/base58"
	local dest="${EXTERNAL_DIR}/base58"
	! [ -e "${dest}" ] || return 0
	[ -e "${tmpdest}" ] || git clone "${url}" "${tmpdest}"
	#
	local wd="${PWD}"
	cd "${tmpdest}"
	git checkout "${commit}"
	cd ${wd}
	# package base58
	mkdir -p "${dest}"
	cp ${tmpdest}/{base58.go,LICENSE} "${dest}/"
}

crypto()
{
	local url="https://go.googlesource.com/crypto"
	local commit="4e0068c0098be10d7025c99ab7c50ce454c1f0f9" # v0.45.0
	local tmpdest="${EXTERNAL_TMP_DIR}/crypto"
	local dest="${EXTERNAL_DIR}/crypto"
	! [ -e "${dest}" ] || return 0
	[ -e "${tmpdest}" ] || git clone "${url}" "${tmpdest}"
	local wd="${PWD}"
	cd "${tmpdest}"
	git checkout "${commit}"
	cd ${wd}
	# package crypto
	mkdir -p "${dest}"
	cp ${tmpdest}/LICENSE "${dest}/"
	# package ripemd160
	mkdir -p "${dest}/ripemd160"
	cp ${tmpdest}/LICENSE "${dest}/"
	cp "${tmpdest}"/ripemd160/{ripemd160.go,ripemd160block.go} \
	"${dest}/ripemd160"
}

secp256k1()
{
	local url="https://github.com/decred/dcrd"
	local commit="f98d08ef138a99711dbbc86c569935ded8d6a986" # dcrec/secp256k1/v4.4.0
	local tmpdest="${EXTERNAL_TMP_DIR}/dcrd"
	local dest="${EXTERNAL_DIR}/dcrd"
	#
	! [ -e "${dest}" ] || return 0
	[ -e "${tmpdest}" ] || git clone "${url}" "${tmpdest}"
	local wd="${PWD}"
	cd "${tmpdest}"
	git checkout "${commit}"
	cd ${wd}
	# package dcrd
	mkdir -p "${dest}"
	cp ${tmpdest}/LICENSE "${dest}/"
	# package blake256
	mkdir -p "${dest}/blake256/internal/_asm"
	cp "${tmpdest}"/crypto/blake256/internal/_asm/{gen_amd64_compress_asm.go,generate.go,README.md} \
	"${dest}"/blake256/internal/_asm/
	#
	mkdir -p "${dest}/blake256/internal/compress"
	cp "${tmpdest}"/crypto/blake256/internal/compress/{blocks_amd64.go,blocks_amd64.s,blocks_generic.go,blocksisa_amd64.go,blocks_noasm.go,cpu_amd64.go,cpu_amd64.s,README.md} \
	"${dest}"/blake256/internal/compress/
	#
	cp -a "${tmpdest}"/crypto/blake256/{error.go,hasher224.go,hasher256.go,hasher.go,README.md} \
	"${dest}"/blake256/
	# package ecdsa
	mkdir -p "${dest}/secp256k1/ecdsa"
	cp -a "${tmpdest}"/dcrec/secp256k1/ecdsa/{doc.go,error.go,signature.go,README.md} \
	"${dest}"/secp256k1/ecdsa
	# package schnorr
	mkdir -p "${dest}/secp256k1/schnorr"
	cp -a "${tmpdest}"/dcrec/secp256k1/schnorr/{doc.go,error.go,pubkey.go,signature.go,README.md} \
	"${dest}"/secp256k1/schnorr/
	# package secp256k1
	cp -a "${tmpdest}"/dcrec/secp256k1/{compressedbytepoints.go,curve_embedded.go,curve.go,curve_precompute.go,doc.go,ecdh.go,ellipticadaptor.go,error.go,field.go,genprecomps.go,loadprecomputed.go,modnscalar.go,nonce.go,privkey.go,pubkey.go,README.md} \
	"${dest}"/secp256k1/
}

replacements()
{
	#github.com/decred/dcrd/crypto/blake256 => github.com/ncodysoftware/eps-go/external/dcrd/blake256
	find ./external -type f -name '*.go' -exec sed -i.bak 's/github.com\/decred\/dcrd\/crypto\/blake256/github.com\/ncodysoftware\/eps-go\/external\/dcrd\/blake256/g' {} \;
	#github.com/decred/dcrd/dcrec/secp256k1/v4 => github.com/ncodysoftware/eps-go/external/dcrd/secp256k1
	find ./external -type f -name '*.go' -exec sed -i.bak 's/github.com\/decred\/dcrd\/dcrec\/secp256k1\/v4/github.com\/ncodysoftware\/eps-go\/external\/dcrd\/secp256k1/g' {} \;
	#
	find ./external -type f -name '*.go.bak' -exec rm {} \;
}

run() 
{
	mkdir -p ${EXTERNAL_TMP_DIR}
	base58
	crypto
	secp256k1
	#
	replacements
}

####################
case "${1}" in
	run) run ;;
	*) eprintln "${HELP_MSG}" ;;
esac
