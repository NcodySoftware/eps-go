package bitcoin

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"time"

	"ncody.com/ncgo.git/stackerr"
)

type Network byte

const (
	Mainnet Network = 0
	Testnet         = 1
	Regtest         = 2
)

const (
	datatype_tx            uint32 = 0x00000001
	datatype_block         uint32 = 0x00000002
	datatype_witness_tx    uint32 = 0x40000001
	datatype_witness_block uint32 = 0x40000002
)

const protoVersion uint32 = 70014

var (
	message_version    = [12]byte{'v', 'e', 'r', 's', 'i', 'o', 'n', 0, 0, 0, 0, 0}
	message_verack     = [12]byte{'v', 'e', 'r', 'a', 'c', 'k', 0, 0, 0, 0, 0, 0}
	message_inv        = [12]byte{'i', 'n', 'v', 0, 0, 0, 0, 0, 0, 0, 0, 0}
	message_tx         = [12]byte{'t', 'x', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	message_block      = [12]byte{'b', 'l', 'o', 'c', 'k', 0, 0, 0, 0, 0, 0, 0}
	message_headers    = [12]byte{'h', 'e', 'a', 'd', 'e', 'r', 's', 0, 0, 0, 0, 0}
	message_ping       = [12]byte{'p', 'i', 'n', 'g', 0, 0, 0, 0, 0, 0, 0, 0}
	message_pong       = [12]byte{'p', 'o', 'n', 'g', 0, 0, 0, 0, 0, 0, 0, 0}
	message_getblocks  = [12]byte{'g', 'e', 't', 'b', 'l', 'o', 'c', 'k', 's', 0, 0, 0}
	message_getheaders = [12]byte{'g', 'e', 't', 'h', 'e', 'a', 'd', 'e', 'r', 's', 0, 0}
	message_getdata    = [12]byte{'g', 'e', 't', 'd', 'a', 't', 'a', 0, 0, 0, 0, 0}
)

var magicBytes = [...][4]byte{
	{0xf9, 0xbe, 0xb4, 0xd9},
	{0x0b, 0x11, 0x09, 0x07},
	{0xfa, 0xbf, 0xb5, 0xda},
}

func NetworkFromString(ns string) Network {
	switch strings.ToLower(ns) {
	case "mainnet":
		return Mainnet
	case "testnet":
		return Testnet
	default:
		return Regtest
	}
}

type message struct {
	MagicBytes [4]byte
	Command    [12]byte
	Size       uint32 // LE
	Checksum   [4]byte
	Payload    []byte
}

func (m *message) Deserialize(r io.Reader) error {
	err := multiErr(
		func() error {
			err := readAll(r, m.MagicBytes[:])
			if err != nil {
				return err
			}
			return nil
		},
		func() error {
			return readAll(r, m.Command[:])
		},
		func() error {
			return uint32ReadLE(r, &m.Size)
		},
		func() error {
			return readAll(r, m.Checksum[:])
		},
		func() error {
			m.Payload = make([]byte, int(m.Size))
			err := readAll(r, m.Payload[:])
			if err != nil {
				return err
			}
			if !checkSumMatch(m.Payload, m.Checksum) {
				return fmt.Errorf("bad payload checksum")
			}
			return nil
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (m *message) Serialize(buf []byte) []byte {
	if bytesIsZero(m.Checksum[:]) {
		m.Checksum = checkSum(m.Payload)
	}
	if bytesIsZero(m.MagicBytes[:]) {
		copy(m.MagicBytes[:], magicBytes[Mainnet][:])
	}
	//
	buf = append(buf, m.MagicBytes[:]...)
	buf = append(buf, m.Command[:]...)
	size := uint32SerializeLE(uint32(len(m.Payload)))
	buf = append(buf, size[:]...)
	buf = append(buf, m.Checksum[:]...)
	buf = append(buf, m.Payload[:]...)
	return buf
}

func (m *message) ToString() string {
	return fmt.Sprintf(
		`magicBytes: %x, cmd: %s, size: %d, checksum: %x, payload: %x`,
		m.MagicBytes,
		m.Command,
		m.Size,
		m.Checksum,
		m.Payload,
	)
}

func makeVersionMessage(magic [4]byte) message {
	var (
		m message
		p version
	)
	m.MagicBytes = magic
	m.Command = message_version
	p.ProtocolVersion = protoVersion
	p.Time = uint64(time.Now().Unix())
	p.UserAgent = []byte("/a_bitcoin_client/")
	p.LastBlock = 0
	m.Payload = p.Serialize(nil)
	return m
}

func makeVerAckMessage(magic [4]byte) message {
	var (
		m message
	)
	m.MagicBytes = magic
	m.Command = message_verack
	return m
}

func makePongMessage(magic [4]byte, nonce [8]byte) message {
	var m message
	m.Command = message_pong
	m.MagicBytes = magic
	m.Payload = nonce[:]
	return m
}

func makePingMessage(magic [4]byte) message {
	var (
		m     message
		nonce [8]byte
	)
	rand.Reader.Read(nonce[:])
	m.Command = message_ping
	m.MagicBytes = magic
	m.Payload = nonce[:]
	return m
}

func makeInvMessage(magic [4]byte, command [12]byte, data inv) message {
	var m message
	m.MagicBytes = magic
	m.Command = command
	m.Payload = data.Serialize(nil)
	return m
}

func makeGetHeadersMessage(
	magic [4]byte, bhashes [][32]byte, end [32]byte,
) message {
	var (
		m message
		p getHeaders
	)
	m.MagicBytes = magic
	m.Command = message_getheaders
	p.Version = protoVersion
	p.BlockHeaderHashes = bhashes
	p.StopHash = end
	m.Payload = p.Serialize(nil)
	return m
}

type version struct {
	ProtocolVersion uint32   // LE
	Services        uint64   // BF LE
	Time            uint64   // LE
	RemoteServices  uint64   // BF LE
	RemoteIP        [16]byte // BE
	RemotePort      uint16   // BE
	LocalServices   uint64   // BF LE
	LocalIP         [16]byte // BE
	LocalPort       uint16   // LE
	Nonce           uint64   // LE
	UserAgent       []byte   // COMPACT SIZE ASCII
	LastBlock       uint32   // LE
}

func (v *version) Deserialize(r io.Reader) error {
	err := multiErr(
		func() error {
			return uint32ReadLE(r, &v.ProtocolVersion)
		},
		func() error {
			return uint64ReadLE(r, &v.Services)
		},
		func() error {
			return uint64ReadLE(r, &v.Time)
		},
		func() error {
			return uint64ReadLE(r, &v.RemoteServices)
		},
		func() error {
			return readAll(r, v.RemoteIP[:])
		},
		func() error {
			return uint16ReadBE(r, &v.RemotePort)
		},
		func() error {
			return uint64ReadLE(r, &v.LocalServices)
		},
		func() error {
			return readAll(r, v.LocalIP[:])
		},
		func() error {
			return uint16ReadBE(r, &v.LocalPort)
		},
		func() error {
			return uint64ReadLE(r, &v.Nonce)
		},
		func() error {
			var uaSize uint64
			err := compactSizeRead(r, &uaSize)
			if err != nil {
				return stackerr.Wrap(err)
			}
			v.UserAgent = make([]byte, int(uaSize))
			return readAll(r, v.UserAgent[:])
		},
		func() error {
			return uint32ReadLE(r, &v.LastBlock)
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (v *version) Serialize(buf []byte) []byte {
	data := struct {
		ProtocolVersion [4]byte  // LE
		Services        [8]byte  // LE
		Time            [8]byte  // LE
		RemoteServices  [8]byte  // LE
		RemoteIP        [16]byte // BE
		RemotePort      [2]byte  // BE
		LocalServices   [8]byte  // LE
		LocalIP         [16]byte // BE
		LocalPort       [2]byte  // LE
		Nonce           [8]byte  // LE
		UserAgent       []byte   // compactsize
		LastBlock       [4]byte  // LE
	}{}
	data.ProtocolVersion = uint32SerializeLE(v.ProtocolVersion)
	data.Services = uint64SerializeLE(v.Services)
	data.Time = uint64SerializeLE(v.Time)
	data.RemoteServices = uint64SerializeLE(v.RemoteServices)
	data.RemoteIP = v.RemoteIP
	data.RemotePort = uint16SerializeBE(v.RemotePort)
	data.LocalServices = uint64SerializeLE(v.LocalServices)
	data.LocalIP = v.LocalIP
	data.LocalPort = uint16SerializeBE(v.LocalPort)
	data.Nonce = uint64SerializeLE(v.Nonce)
	data.UserAgent = compactSizeSerialize(uint64(len(v.UserAgent)))
	data.UserAgent = append(data.UserAgent, v.UserAgent...)
	data.LastBlock = uint32SerializeLE(v.LastBlock)
	buf = buf[:0]
	buf = append(buf, data.ProtocolVersion[:]...)
	buf = append(buf, data.Services[:]...)
	buf = append(buf, data.Time[:]...)
	buf = append(buf, data.RemoteServices[:]...)
	buf = append(buf, data.RemoteIP[:]...)
	buf = append(buf, data.RemotePort[:]...)
	buf = append(buf, data.LocalServices[:]...)
	buf = append(buf, data.LocalIP[:]...)
	buf = append(buf, data.LocalPort[:]...)
	buf = append(buf, data.Nonce[:]...)
	buf = append(buf, data.UserAgent[:]...)
	buf = append(buf, data.LastBlock[:]...)
	return buf
}

// Used for inv and getdata commands
type inv struct {
	Count     uint64 // compactsize
	Inventory []inventory
}

func (i *inv) Deserialize(r io.Reader) error {
	var (
		err error
	)
	err = compactSizeRead(r, &i.Count)
	if err != nil {
		return stackerr.Wrap(err)
	}
	i.Inventory = make([]inventory, 0, int(i.Count))
	for range i.Count {
		var d inventory
		err = d.Deserialize(r)
		if err != nil {
			return stackerr.Wrap(err)
		}
		i.Inventory = append(i.Inventory, d)
	}
	return nil
}

func (i *inv) Serialize(buf []byte) []byte {
	count := compactSizeSerialize(uint64(len(i.Inventory)))
	buf = append(buf, count[:]...)
	for _, d := range i.Inventory {
		buf = d.Serialize(buf)
	}
	return buf
}

type inventory struct {
	Type uint32 // LE
	Hash [32]byte
}

func (i *inventory) Deserialize(r io.Reader) error {
	err := multiErr(
		func() error {
			return uint32ReadLE(r, &i.Type)
		},
		func() error {
			return readAll(r, i.Hash[:])
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (i *inventory) Serialize(buf []byte) []byte {
	d := struct {
		Type [4]byte // le
		Hash [32]byte
	}{}
	d.Type = uint32SerializeLE(i.Type)
	d.Hash = i.Hash
	buf = append(buf, d.Type[:]...)
	buf = append(buf, d.Hash[:]...)
	return buf
}

type ping struct {
	Nonce [8]byte
}

func (p *ping) Deserialize(r io.Reader) error {
	err := readAll(r, p.Nonce[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (p *ping) Serialize(buf []byte) []byte {
	buf = append(buf, p.Nonce[:]...)
	return buf
}

type getHeaders struct {
	Version           uint32     // LE
	HashCount         uint64     // compactsize
	BlockHeaderHashes [][32]byte // high height first
	StopHash          [32]byte   // last header (can be all zeroes)
}

func (h *getHeaders) Deserialize(r io.Reader) error {
	err := multiErr(
		func() error {
			return uint32ReadLE(r, &h.Version)
		},
		func() error {
			return compactSizeRead(r, &h.HashCount)
		},
		func() error {
			h.BlockHeaderHashes = make([][32]byte, h.HashCount)
			for i := range h.HashCount {
				err := readAll(r, h.BlockHeaderHashes[i][:])
				if err != nil {
					return stackerr.Wrap(err)
				}
			}
			return nil
		},
		func() error {
			return readAll(r, h.StopHash[:])
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (h *getHeaders) Serialize(buf []byte) []byte {
	v := uint32SerializeLE(h.Version)
	buf = append(buf, v[:]...)
	hlen := compactSizeSerialize(uint64(len(h.BlockHeaderHashes)))
	buf = append(buf, hlen...)
	for _, h := range h.BlockHeaderHashes {
		buf = append(buf, h[:]...)
	}
	buf = append(buf, h.StopHash[:]...)
	return buf
}

type headers struct {
	Count   uint64 // compactsize
	Headers []Header
}

func (h *headers) Deserialize(r io.Reader) error {
	err := compactSizeRead(r, &h.Count)
	if err != nil {
		return stackerr.Wrap(err)
	}
	h.Headers = make([]Header, h.Count)
	for i := range h.Count {
		err = h.Headers[i].Deserialize(r)
		if err != nil {
			return stackerr.Wrap(err)
		}
	}
	return nil
}

func (h *headers) Serialize(buf []byte) []byte {
	count := compactSizeSerialize(uint64(len(h.Headers)))
	buf = append(buf, count...)
	for _, h := range h.Headers {
		buf = h.Serialize(buf)
	}
	return buf
}

type Header struct {
	BlockVersion  uint32 // LE
	PreviousBlock [32]byte
	MerkleRoot    [32]byte
	Timestamp     uint32 // LE
	NBits         uint32 // LE
	Nonce         uint32 // LE
}

func (h *Header) Deserialize(r io.Reader) error {
	err := multiErr(
		func() error {
			return uint32ReadLE(r, &h.BlockVersion)
		},
		func() error {
			return readAll(r, h.PreviousBlock[:])
		},
		func() error {
			return readAll(r, h.MerkleRoot[:])
		},
		func() error {
			return uint32ReadLE(r, &h.Timestamp)
		},
		func() error {
			return uint32ReadLE(r, &h.NBits)
		},
		func() error {
			return uint32ReadLE(r, &h.Nonce)
		},
		func() error {
			var buf [1]byte
			return readAll(r, buf[:])
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (h *Header) Serialize(buf []byte) []byte {
	v := uint32SerializeLE(h.BlockVersion)
	buf = append(buf, v[:]...)
	buf = append(buf, h.PreviousBlock[:]...)
	buf = append(buf, h.MerkleRoot[:]...)
	ts := uint32SerializeLE(h.Timestamp)
	buf = append(buf, ts[:]...)
	b := uint32SerializeLE(h.NBits)
	buf = append(buf, b[:]...)
	n := uint32SerializeLE(h.Nonce)
	buf = append(buf, n[:]...)
	buf = append(buf, 0)
	return buf
}

func (h *Header) ToString() string {
	return fmt.Sprintf(
		"header: blkv: %d; prev: %x; merk: %x; time %d; NBits: %x; Nonce: %d",
		h.BlockVersion,
		h.PreviousBlock,
		h.MerkleRoot,
		h.Timestamp,
		h.NBits,
		h.Nonce,
	)
}

func (h *Header) Hash(buf *[]byte) [32]byte {
	var (
		buf2 []byte
	)
	if buf == nil {
		buf = &buf2
	}
	*buf = (*buf)[:0]
	ver := uint32SerializeLE(h.BlockVersion)
	*buf = append(*buf, ver[:]...)
	*buf = append(*buf, h.PreviousBlock[:]...)
	*buf = append(*buf, h.MerkleRoot[:]...)
	t := uint32SerializeLE(h.Timestamp)
	*buf = append(*buf, t[:]...)
	bits := uint32SerializeLE(h.NBits)
	*buf = append(*buf, bits[:]...)
	nonce := uint32SerializeLE(h.Nonce)
	*buf = append(*buf, nonce[:]...)
	hs := sha256.Sum256(*buf)
	hs = sha256.Sum256(hs[:])
	return hs
}

type Block struct {
	Version          uint32 // LE
	PreviousBlock    [32]byte
	MerkleRoot       [32]byte
	Time             uint32 // LE
	Bits             uint32 // LE
	Nonce            uint32 // LE
	TransactionCount uint64 // compactsize
	Transactions     []Transaction
}

func (b *Block) Deserialize(r io.Reader) error {
	err := multiErr(
		func() error {
			return uint32ReadLE(r, &b.Version)
		},
		func() error {
			return readAll(r, b.PreviousBlock[:])
		},
		func() error {
			return readAll(r, b.MerkleRoot[:])
		},
		func() error {
			return uint32ReadLE(r, &b.Time)
		},
		func() error {
			return uint32ReadLE(r, &b.Bits)
		},
		func() error {
			return uint32ReadLE(r, &b.Nonce)
		},
		func() error {
			return compactSizeRead(r, &b.TransactionCount)
		},
		func() error {
			b.Transactions = make(
				[]Transaction, 0, b.TransactionCount,
			)
			for range b.TransactionCount {
				var t Transaction
				err := t.Deserialize(r)
				if err != nil {
					return stackerr.Wrap(err)
				}
				b.Transactions = append(b.Transactions, t)
			}
			return nil
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (b *Block) Serialize(buf []byte) []byte {
	ver := uint32SerializeLE(b.Version)
	buf = append(buf, ver[:]...)
	buf = append(buf, b.PreviousBlock[:]...)
	buf = append(buf, b.MerkleRoot[:]...)
	time := uint32SerializeLE(b.Time)
	buf = append(buf, time[:]...)
	bits := uint32SerializeLE(b.Bits)
	buf = append(buf, bits[:]...)
	nonce := uint32SerializeLE(b.Nonce)
	buf = append(buf, nonce[:]...)
	txc := compactSizeSerialize(uint64(len(b.Transactions)))
	buf = append(buf, txc...)
	for _, t := range b.Transactions {
		buf = t.Serialize(buf)
	}
	return buf
}

func (b *Block) Hash(buf *[]byte) [32]byte {
	var (
		buf2 []byte
	)
	if buf == nil {
		buf = &buf2
	}
	*buf = (*buf)[:0]
	ver := uint32SerializeLE(b.Version)
	*buf = append(*buf, ver[:]...)
	*buf = append(*buf, b.PreviousBlock[:]...)
	*buf = append(*buf, b.MerkleRoot[:]...)
	t := uint32SerializeLE(b.Time)
	*buf = append(*buf, t[:]...)
	bits := uint32SerializeLE(b.Bits)
	*buf = append(*buf, bits[:]...)
	nonce := uint32SerializeLE(b.Nonce)
	*buf = append(*buf, nonce[:]...)
	h := sha256.Sum256(*buf)
	h = sha256.Sum256(h[:])
	return h
}

func (b *Block) TXIDs(txidBuf [][32]byte, buf *[]byte) [][32]byte {
	for i := range b.Transactions {
		*buf = (*buf)[:0]
		txid := b.Transactions[i].Txid(buf)
		txidBuf = append(txidBuf, txid)
	}
	return txidBuf
}

type Transaction struct {
	Version     uint32 // LE
	Marker      byte
	Flag        byte
	InputCount  uint64 // compactsize
	Inputs      []Input
	OutputCount uint64 // compactsize
	Outputs     []Output
	Witness     []Witness
	Locktime    uint32
}

func (t *Transaction) Deserialize(r io.Reader) error {
	isSegwit := false
	err := multiErr(
		func() error {
			return uint32ReadLE(r, &t.Version)
		},
		func() error {
			var markerOrVinCount uint64
			err := compactSizeRead(r, &markerOrVinCount)
			if err != nil {
				return stackerr.Wrap(err)
			}
			if markerOrVinCount == 0 {
				isSegwit = true
				return nil
			}
			t.InputCount = markerOrVinCount
			return nil
		},
		func() error {
			if !isSegwit {
				return nil
			}
			var buf = [1]byte{}
			n, err := r.Read(buf[:])
			if err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("bad length")
			}
			t.Flag = buf[0]
			if t.Flag != 0x01 {
				return fmt.Errorf("bad flag")
			}
			return nil
		},
		func() error {
			if !isSegwit {
				return nil
			}
			return compactSizeRead(r, &t.InputCount)
		},
		func() error {
			t.Inputs = make([]Input, 0, t.InputCount)
			for range t.InputCount {
				var i Input
				err := i.Deserialize(r)
				if err != nil {
					return err
				}
				t.Inputs = append(t.Inputs, i)
			}
			return nil
		},
		func() error {
			return compactSizeRead(r, &t.OutputCount)
		},
		func() error {
			t.Outputs = make([]Output, 0, t.OutputCount)
			for range t.OutputCount {
				var o Output
				err := o.Deserialize(r)
				if err != nil {
					return err
				}
				t.Outputs = append(t.Outputs, o)
			}
			return nil
		},
		func() error {
			if !isSegwit {
				return nil
			}
			t.Witness = make([]Witness, 0, t.InputCount)
			for range t.InputCount {
				var w Witness
				err := w.Deserialize(r)
				if err != nil {
					return err
				}
				t.Witness = append(t.Witness, w)
			}
			return nil
		},
		func() error {
			return uint32ReadLE(r, &t.Locktime)
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (t *Transaction) Serialize(buf []byte) []byte {
	isSegwit := t.Flag == 0x01
	vers := uint32SerializeLE(t.Version)
	buf = append(buf, vers[:]...)
	if isSegwit {
		buf = append(buf, t.Marker)
		buf = append(buf, t.Flag)
	}
	icount := compactSizeSerialize(uint64(len(t.Inputs)))
	buf = append(buf, icount...)
	for _, i := range t.Inputs {
		buf = i.Serialize(buf)
	}
	ocount := compactSizeSerialize(uint64(len(t.Outputs)))
	buf = append(buf, ocount...)
	for _, o := range t.Outputs {
		buf = o.Serialize(buf)
	}
	if isSegwit {
		for _, w := range t.Witness {
			buf = w.Serialize(buf)
		}
	}
	lt := uint32SerializeLE(t.Locktime)
	buf = append(buf, lt[:]...)
	return buf
}

func (t *Transaction) serializeForTxid(buf []byte) []byte {
	vers := uint32SerializeLE(t.Version)
	buf = append(buf, vers[:]...)
	icount := compactSizeSerialize(uint64(len(t.Inputs)))
	buf = append(buf, icount...)
	for _, i := range t.Inputs {
		buf = i.Serialize(buf)
	}
	ocount := compactSizeSerialize(uint64(len(t.Outputs)))
	buf = append(buf, ocount...)
	for _, o := range t.Outputs {
		buf = o.Serialize(buf)
	}
	lt := uint32SerializeLE(t.Locktime)
	buf = append(buf, lt[:]...)
	return buf

}

func (t *Transaction) Txid(buf *[]byte) [32]byte {
	var buf2 []byte
	if buf == nil {
		buf = &buf2
	}
	*buf = (*buf)[:0]
	serialized := t.serializeForTxid(*buf)
	*buf = serialized
	h := sha256.Sum256(serialized)
	h = sha256.Sum256(h[:])
	return h
}

type Input struct {
	Txid          [32]byte
	Vout          uint32 // LE
	ScriptSigSize uint64 // compactsize
	ScriptSig     []byte
	Sequence      uint32 // LE
}

func (i *Input) Deserialize(r io.Reader) error {
	err := multiErr(
		func() error {
			return readAll(r, i.Txid[:])
		},
		func() error {
			return uint32ReadLE(r, &i.Vout)
		},
		func() error {
			return compactSizeRead(r, &i.ScriptSigSize)
		},
		func() error {
			i.ScriptSig = make([]byte, i.ScriptSigSize)
			return readAll(r, i.ScriptSig)
		},
		func() error {
			return uint32ReadLE(r, &i.Sequence)
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (i *Input) Serialize(buf []byte) []byte {
	/*
		Txid [32]byte
		Vout uint32 // LE
		ScriptSigSize uint64 // compactsize
		ScriptSig []byte
		Sequence uint32 // LE
	*/
	buf = append(buf, i.Txid[:]...)
	vout := uint32SerializeLE(i.Vout)
	buf = append(buf, vout[:]...)
	buf = append(buf, compactSizeSerialize(uint64(len(i.ScriptSig)))...)
	buf = append(buf, i.ScriptSig...)
	sequence := uint32SerializeLE(i.Sequence)
	buf = append(buf, sequence[:]...)
	return buf
}

type Output struct {
	Amount           uint64 // LE
	ScriptPubKeySize uint64 // compactsize
	ScriptPubkey     []byte
}

func (o *Output) Deserialize(r io.Reader) error {
	err := multiErr(
		func() error {
			return uint64ReadLE(r, &o.Amount)
		},
		func() error {
			return compactSizeRead(r, &o.ScriptPubKeySize)
		},
		func() error {
			o.ScriptPubkey = make([]byte, o.ScriptPubKeySize)
			return readAll(r, o.ScriptPubkey)
		},
	)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (o *Output) Serialize(buf []byte) []byte {
	amount := uint64SerializeLE(o.Amount)
	buf = append(buf, amount[:]...)
	spks := compactSizeSerialize(uint64(len(o.ScriptPubkey)))
	buf = append(buf, spks...)
	buf = append(buf, o.ScriptPubkey...)
	return buf
}

type Witness struct {
	StackItemCount uint64 //compactsize
	StackItems     []stackItem
}

func (w *Witness) Deserialize(r io.Reader) error {
	err := compactSizeRead(r, &w.StackItemCount)
	if err != nil {
		return stackerr.Wrap(err)
	}
	w.StackItems = make([]stackItem, 0, w.StackItemCount)
	for range w.StackItemCount {
		var s stackItem
		err := s.Deserialize(r)
		if err != nil {
			return stackerr.Wrap(err)
		}
		w.StackItems = append(w.StackItems, s)
	}
	return nil
}

func (w *Witness) Serialize(buf []byte) []byte {
	count := compactSizeSerialize(uint64(len(w.StackItems)))
	buf = append(buf, count...)
	for _, item := range w.StackItems {
		buf = item.Serialize(buf)
	}
	return buf
}

type stackItem struct {
	Size uint64 // compactsize
	Item []byte
}

func (s *stackItem) Deserialize(r io.Reader) error {
	err := compactSizeRead(r, &s.Size)
	if err != nil {
		return stackerr.Wrap(err)
	}
	s.Item = make([]byte, s.Size)
	err = readAll(r, s.Item)
	if err != nil {
		return stackerr.Wrap(err)
	}
	return nil
}

func (s *stackItem) Serialize(buf []byte) []byte {
	size := compactSizeSerialize(uint64(len(s.Item)))
	buf = append(buf, size...)
	buf = append(buf, s.Item...)
	return buf
}

type readWriteCloser interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}

func multiErr(funcs ...func() error) error {
	for i, f := range funcs {
		err := f()
		if err == nil {
			continue
		}
		return fmt.Errorf("multierr index %d: %w", i, err)
	}
	return nil
}

func readAll(r io.Reader, dest []byte) error {
	var (
		count, tmp int
		err        error
		target     = len(dest)
	)
	for count < target {
		tmp, err = r.Read(dest[count:])
		if err != nil {
			return stackerr.Wrap(err)
		}
		count += tmp
	}
	return nil
}

func writeAll(w io.Writer, src []byte) error {
	var (
		count, tmp int
		err        error
		target     = len(src)
	)
	for count < target {
		tmp, err = w.Write(src)
		if err != nil {
			return stackerr.Wrap(err)
		}
		count += tmp
	}
	return nil
}

func compactSizeSerialize(s uint64) []byte {
	var r [9]byte
	switch {
	case s <= 0xFC:
		r[0] = byte(s)
		return r[:1]
	case s <= uint64(^uint16(0)):
		r[0] = 0xFD
		u := uint16SerializeLE(uint16(s))
		copy(r[1:], u[:])
		return r[:1+2]
	case s <= uint64(^uint32(0)):
		r[0] = 0xFE
		u := uint32SerializeLE(uint32(s))
		copy(r[1:], u[:])
		return r[:1+4]
	default:
		r[0] = 0xFF
		u := uint64SerializeLE(s)
		copy(r[1:], u[:])
		return r[:1+8]
	}
}

func compactSizeDeserialize(s []byte) uint64 {
	if len(s) < 1 {
		return 0
	}
	switch s[0] {
	case byte(0xFD):
		if len(s) < 3 {
			return 0
		}
		return uint64(uint16DeserializeLE([2]byte(s[1 : 1+2])))
	case byte(0xFE):
		if len(s) < 5 {
			return 0
		}
		return uint64(uint32DeserializeLE([4]byte(s[1 : 1+4])))
	case byte(0xFF):
		if len(s) < 9 {
			return 0
		}
		return uint64DeserializeLE([8]byte(s[1 : 1+8]))
	default:
		return uint64(s[0])
	}
}

func compactSizeRead(r io.Reader, dest *uint64) error {
	const s = 9
	var buf [s]byte
	n, err := r.Read(buf[:1])
	if err != nil {
		return stackerr.Wrap(err)
	}
	if n != 1 {
		return fmt.Errorf("bad read size")
	}
	var target int = 0
	switch buf[0] {
	case 0xFD:
		target = 2
	case 0xFE:
		target = 4
	case 0xFF:
		target = 8
	default:
		*dest = uint64(buf[0])
		return nil
	}
	n, err = r.Read(buf[1 : 1+target])
	if err != nil {
		return stackerr.Wrap(err)
	}
	if n != target {
		return fmt.Errorf("bad read size")
	}
	*dest = compactSizeDeserialize(buf[:])
	return nil
}

func uint16SerializeLE(v uint16) [2]byte {
	var r [2]byte
	r[1] = byte(v >> (8 * 1))
	r[0] = byte(v >> (8 * 0))
	return r
}

func uint16DeserializeLE(v [2]byte) uint16 {
	var r uint16
	r |= uint16(v[1])
	r <<= 8
	r |= uint16(v[0])
	return r
}

func uint16SerializeBE(v uint16) [2]byte {
	var r [2]byte
	r[0] = byte(v >> (8 * 1))
	r[1] = byte(v >> (8 * 0))
	return r
}

func uint16DeserializeBE(v [2]byte) uint16 {
	var r uint16
	r |= uint16(v[0])
	r <<= 8
	r |= uint16(v[1])
	return r
}

func uint16ReadBE(r io.Reader, dest *uint16) error {
	const s = 2
	var buf [s]byte
	err := readAll(r, buf[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	*dest = uint16DeserializeBE(buf)
	return nil
}

func uint32SerializeLE(v uint32) [4]byte {
	var r [4]byte
	r[3] = byte(v >> (8 * 3))
	r[2] = byte(v >> (8 * 2))
	r[1] = byte(v >> (8 * 1))
	r[0] = byte(v >> (8 * 0))
	return r
}

func uint32DeserializeLE(v [4]byte) uint32 {
	var r uint32
	r |= uint32(v[3])
	r <<= 8
	r |= uint32(v[2])
	r <<= 8
	r |= uint32(v[1])
	r <<= 8
	r |= uint32(v[0])
	return r
}

func uint32ReadLE(r io.Reader, dest *uint32) error {
	const s = 4
	var buf [s]byte
	err := readAll(r, buf[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	*dest = uint32DeserializeLE(buf)
	return nil
}

func uint32SerializeBE(v uint32) [4]byte {
	var r [4]byte
	r[0] = byte(v >> (8 * 3))
	r[1] = byte(v >> (8 * 2))
	r[2] = byte(v >> (8 * 1))
	r[3] = byte(v >> (8 * 0))
	return r
}

func uint32DeserializeBE(v [4]byte) uint32 {
	var r uint32
	r |= uint32(v[0])
	r <<= 8
	r |= uint32(v[1])
	r <<= 8
	r |= uint32(v[2])
	r <<= 8
	r |= uint32(v[3])
	return r
}

func uint64SerializeLE(v uint64) [8]byte {
	var r [8]byte
	r[7] = byte(v >> (8 * 7))
	r[6] = byte(v >> (8 * 6))
	r[5] = byte(v >> (8 * 5))
	r[4] = byte(v >> (8 * 4))
	r[3] = byte(v >> (8 * 3))
	r[2] = byte(v >> (8 * 2))
	r[1] = byte(v >> (8 * 1))
	r[0] = byte(v >> (8 * 0))
	return r
}

func uint64DeserializeLE(v [8]byte) uint64 {
	var r uint64
	r |= uint64(v[7])
	r <<= 8
	r |= uint64(v[6])
	r <<= 8
	r |= uint64(v[5])
	r <<= 8
	r |= uint64(v[4])
	r <<= 8
	r |= uint64(v[3])
	r <<= 8
	r |= uint64(v[2])
	r <<= 8
	r |= uint64(v[1])
	r <<= 8
	r |= uint64(v[0])
	return r
}

func uint64ReadLE(r io.Reader, dest *uint64) error {
	const s = 8
	var buf [s]byte
	err := readAll(r, buf[:])
	if err != nil {
		return stackerr.Wrap(err)
	}
	*dest = uint64DeserializeLE(buf)
	return nil
}

func uint64SerializeBE(v uint64) [8]byte {
	var r [8]byte
	r[0] = byte(v >> (8 * 7))
	r[1] = byte(v >> (8 * 6))
	r[2] = byte(v >> (8 * 5))
	r[3] = byte(v >> (8 * 4))
	r[4] = byte(v >> (8 * 3))
	r[5] = byte(v >> (8 * 2))
	r[6] = byte(v >> (8 * 1))
	r[7] = byte(v >> (8 * 0))
	return r
}

func uint64DeserializeBE(v [8]byte) uint64 {
	var r uint64
	r |= uint64(v[0])
	r <<= 8
	r |= uint64(v[1])
	r <<= 8
	r |= uint64(v[2])
	r <<= 8
	r |= uint64(v[3])
	r <<= 8
	r |= uint64(v[4])
	r <<= 8
	r |= uint64(v[5])
	r <<= 8
	r |= uint64(v[6])
	r <<= 8
	r |= uint64(v[7])
	return r
}

func bytesIsZero(b []byte) bool {
	zero := true
	for _, _b := range b {
		if _b != 0 {
			zero = false
			break
		}
	}
	return zero
}

func checkSum(data []byte) [4]byte {
	h := sha256.Sum256(data)
	h1 := sha256.Sum256(h[:])
	return [4]byte(h1[:4])
}

func checkSumMatch(data []byte, c [4]byte) bool {
	h := checkSum(data)
	return bytes.Equal(c[:], h[:4])
}
