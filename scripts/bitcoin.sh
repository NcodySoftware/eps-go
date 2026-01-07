#!/usr/bin/env bash
####################
set -e
####################
readonly RELDIR="$(dirname ${0})"
readonly HELP_MSG='usage: < set-state | help >'
####################
eprintln()
{
	! [ -z "${1}" ] || eprintln "eprintln: missing message"
	printf "${1}\n" 1>&2
	exit 1
}
eps_wallet() 
{
	../eps-pod/control.sh down || true
	../bitcoind-pod/control.sh bitcoin-cli "--named createwallet wallet_name=eps disable_private_keys=true blank=true avoid_reuse=false descriptors=false load_on_startup=true" 1>/dev/null
	../eps-pod/control.sh up 1>/dev/null 2>&1
	sleep 10
	../eps-pod/control.sh down || true 1>/dev/null 2>&1
	../bitcoind-pod/control.sh bitcoin-cli '-rpcwallet=eps rescanblockchain'
	../eps-pod/control.sh up 1>/dev/null 2>&1
}
tmp_rawtx()
{
	local change_0=bcrt1qyhrahst409dtjzndfyp08gg8e7x0hx7vr867nm

	../bitcoind-pod/control.sh bitcoin-cli '-rpcwallet=test --named listunspent' > /tmp/eps-go/utxos
	local txid=$(cat /tmp/eps-go/utxos | jq -r .[0].txid)
	local vout=$(cat /tmp/eps-go/utxos | jq -r .[0].vout)
	local unsigned=$(../bitcoind-pod/control.sh bitcoin-cli "--named createrawtransaction inputs=[{\"txid\":\"${txid}\",\"vout\":${vout}}] outputs=[{\"${change_0}\":\"48\"}]") 1>/dev/null

	../bitcoind-pod/control.sh bitcoin-cli "-rpcwallet=test signrawtransactionwithwallet ${unsigned}" > /tmp/eps-go/signed

	local signed=$(cat /tmp/eps-go/signed | jq -r .hex)
	printf "rawtx: ${signed}\n"
}
set_state()
{
	! [ -e /tmp/eps-go ] || rm -r /tmp/eps-go
	mkdir -p /tmp/eps-go
	../bitcoind-pod/control.sh down 1>/dev/null 2>&1 || true
	rm -rf ../bitcoind-pod/data
	../bitcoind-pod/control.sh up 1>/dev/null 2>&1
	sleep 5

	../bitcoind-pod/control.sh bitcoin-cli "--named createwallet wallet_name=test disable_private_keys=false blank=true descriptors=true load_on_startup=true" 1>/dev/null

	../bitcoind-pod/control.sh bitcoin-cli "-rpcwallet=test --named importdescriptors requests=[{\"desc\":\"wpkh(tprv8ZgxMBicQKsPcu2RP5wqPykWQxf9BpX75RnRKfyVK8b2BrpBcaT8Ae7vh6q9aNWHC2piSXQatuwKdzVvrNNKwxbnLoJtcDzcNaQCEZedNdJ/84h/0h/0h/0/*)#f03g2f34\",\"active\":true,\"range\":[0,0],\"next_index\":0,\"timestamp\":\"now\",\"internal\":false},{\"desc\":\"wpkh(tprv8ZgxMBicQKsPcu2RP5wqPykWQxf9BpX75RnRKfyVK8b2BrpBcaT8Ae7vh6q9aNWHC2piSXQatuwKdzVvrNNKwxbnLoJtcDzcNaQCEZedNdJ/84h/0h/0h/1/*)#cm5fhupd\",\"active\":true,\"range\":[0,0],\"next_index\":0,\"timestamp\":\"now\",\"internal\":true}]" 1>/dev/null

	local recv_0=bcrt1q56fzm5fmj7wtl5cs2n9ez09mw5yxq9n4ahwv7n
	local change_0=bcrt1qyhrahst409dtjzndfyp08gg8e7x0hx7vr867nm
	local drain=bcrt1q5xzpm0vwj02yxzyepks42ezshjql2vnnym7h3t

	../bitcoind-pod/control.sh bitcoin-cli "generatetoaddress 101 ${recv_0}" 1>/dev/null

	../bitcoind-pod/control.sh bitcoin-cli '-rpcwallet=test --named listunspent' > /tmp/eps-go/utxos
	local txid=$(cat /tmp/eps-go/utxos | jq -r .[0].txid)
	local vout=$(cat /tmp/eps-go/utxos | jq -r .[0].vout)

	local unsigned=$(../bitcoind-pod/control.sh bitcoin-cli "--named createrawtransaction inputs=[{\"txid\":\"${txid}\",\"vout\":${vout}}] outputs=[{\"${change_0}\":\"49\"}]") 1>/dev/null

	../bitcoind-pod/control.sh bitcoin-cli "-rpcwallet=test signrawtransactionwithwallet ${unsigned}" > /tmp/eps-go/signed

	local signed=$(cat /tmp/eps-go/signed | jq -r .hex)

	../bitcoind-pod/control.sh bitcoin-cli "--named sendrawtransaction hexstring=${signed} maxfeerate=0" 1>/dev/null

	eps_wallet

	../bitcoind-pod/control.sh bitcoin-cli "generatetoaddress 1 ${drain}" 1>/dev/null
	touch /tmp/eps-go/bitcoind-ok
}
####################
case "${1}" in
	set-state) set_state ;;
	*) eprintln "${HELP_MSG}" ;;
esac
