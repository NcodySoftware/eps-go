package bip32

import (
	"github.com/ncodysoftware/eps-go/assert"
	"github.com/ncodysoftware/eps-go/testutil"
	"reflect"
	"testing"
)

func TestBip32_vector1(t *testing.T) {
	seed := [16]byte(testutil.MustHexDecode("000102030405060708090a0b0c0d0e0f"))
	tests := []struct {
		derivationPath string
		expXpub        string
		expXpriv       string
	}{
		{
			derivationPath: "m",
			expXpub:        "xpub661MyMwAqRbcFtXgS5sYJABqqG9YLmC4Q1Rdap9gSE8NqtwybGhePY2gZ29ESFjqJoCu1Rupje8YtGqsefD265TMg7usUDFdp6W1EGMcet8",
			expXpriv:       "xprv9s21ZrQH143K3QTDL4LXw2F7HEK3wJUD2nW2nRk4stbPy6cq3jPPqjiChkVvvNKmPGJxWUtg6LnF5kejMRNNU3TGtRBeJgk33yuGBxrMPHi",
		},
		{
			derivationPath: "m/0H",
			expXpub:        "xpub68Gmy5EdvgibQVfPdqkBBCHxA5htiqg55crXYuXoQRKfDBFA1WEjWgP6LHhwBZeNK1VTsfTFUHCdrfp1bgwQ9xv5ski8PX9rL2dZXvgGDnw",
			expXpriv:       "xprv9uHRZZhk6KAJC1avXpDAp4MDc3sQKNxDiPvvkX8Br5ngLNv1TxvUxt4cV1rGL5hj6KCesnDYUhd7oWgT11eZG7XnxHrnYeSvkzY7d2bhkJ7",
		},
		{
			derivationPath: "m/0H/1",
			expXpub:        "xpub6ASuArnXKPbfEwhqN6e3mwBcDTgzisQN1wXN9BJcM47sSikHjJf3UFHKkNAWbWMiGj7Wf5uMash7SyYq527Hqck2AxYysAA7xmALppuCkwQ",
			expXpriv:       "xprv9wTYmMFdV23N2TdNG573QoEsfRrWKQgWeibmLntzniatZvR9BmLnvSxqu53Kw1UmYPxLgboyZQaXwTCg8MSY3H2EU4pWcQDnRnrVA1xe8fs",
		},
		{
			derivationPath: "m/0H/1/2H",
			expXpub:        "xpub6D4BDPcP2GT577Vvch3R8wDkScZWzQzMMUm3PWbmWvVJrZwQY4VUNgqFJPMM3No2dFDFGTsxxpG5uJh7n7epu4trkrX7x7DogT5Uv6fcLW5",
			expXpriv:       "xprv9z4pot5VBttmtdRTWfWQmoH1taj2axGVzFqSb8C9xaxKymcFzXBDptWmT7FwuEzG3ryjH4ktypQSAewRiNMjANTtpgP4mLTj34bhnZX7UiM",
		},
		{
			derivationPath: "m/0H/1/2H/2",
			expXpub:        "xpub6FHa3pjLCk84BayeJxFW2SP4XRrFd1JYnxeLeU8EqN3vDfZmbqBqaGJAyiLjTAwm6ZLRQUMv1ZACTj37sR62cfN7fe5JnJ7dh8zL4fiyLHV",
			expXpriv:       "xprvA2JDeKCSNNZky6uBCviVfJSKyQ1mDYahRjijr5idH2WwLsEd4Hsb2Tyh8RfQMuPh7f7RtyzTtdrbdqqsunu5Mm3wDvUAKRHSC34sJ7in334",
		},
		{
			derivationPath: "m/0H/1/2H/2/1000000000",
			expXpub:        "xpub6H1LXWLaKsWFhvm6RVpEL9P4KfRZSW7abD2ttkWP3SSQvnyA8FSVqNTEcYFgJS2UaFcxupHiYkro49S8yGasTvXEYBVPamhGW6cFJodrTHy",
			expXpriv:       "xprvA41z7zogVVwxVSgdKUHDy1SKmdb533PjDz7J6N6mV6uS3ze1ai8FHa8kmHScGpWmj4WggLyQjgPie1rFSruoUihUZREPSL39UNdE3BBDu76",
		},
	}
	derive := func(seed []byte, path string) (string, string, error) {
		return FromSeed(seed, path)
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			xpub, xpriv, err := derive(seed[:], test.derivationPath)
			assert.Must(t, err)
			mustEqualEncodedExt(t, test.expXpub, xpub)
			mustEqualEncodedExt(t, test.expXpriv, xpriv)
		})
	}
}

func TestBip32_vector2(t *testing.T) {
	seed := testutil.MustHexDecode("fffcf9f6f3f0edeae7e4e1dedbd8d5d2cfccc9c6c3c0bdbab7b4b1aeaba8a5a29f9c999693908d8a8784817e7b7875726f6c696663605d5a5754514e4b484542")
	tests := []struct {
		derivationPath string
		expXpub        string
		expXpriv       string
	}{
		{
			derivationPath: "m",
			expXpub:        "xpub661MyMwAqRbcFW31YEwpkMuc5THy2PSt5bDMsktWQcFF8syAmRUapSCGu8ED9W6oDMSgv6Zz8idoc4a6mr8BDzTJY47LJhkJ8UB7WEGuduB",
			expXpriv:       "xprv9s21ZrQH143K31xYSDQpPDxsXRTUcvj2iNHm5NUtrGiGG5e2DtALGdso3pGz6ssrdK4PFmM8NSpSBHNqPqm55Qn3LqFtT2emdEXVYsCzC2U",
		},
		{
			derivationPath: "m/0",
			expXpub:        "xpub69H7F5d8KSRgmmdJg2KhpAK8SR3DjMwAdkxj3ZuxV27CprR9LgpeyGmXUbC6wb7ERfvrnKZjXoUmmDznezpbZb7ap6r1D3tgFxHmwMkQTPH",
			expXpriv:       "xprv9vHkqa6EV4sPZHYqZznhT2NPtPCjKuDKGY38FBWLvgaDx45zo9WQRUT3dKYnjwih2yJD9mkrocEZXo1ex8G81dwSM1fwqWpWkeS3v86pgKt",
		},
		{
			derivationPath: "m/0/2147483647H",
			expXpub:        "xpub6ASAVgeehLbnwdqV6UKMHVzgqAG8Gr6riv3Fxxpj8ksbH9ebxaEyBLZ85ySDhKiLDBrQSARLq1uNRts8RuJiHjaDMBU4Zn9h8LZNnBC5y4a",
			expXpriv:       "xprv9wSp6B7kry3Vj9m1zSnLvN3xH8RdsPP1Mh7fAaR7aRLcQMKTR2vidYEeEg2mUCTAwCd6vnxVrcjfy2kRgVsFawNzmjuHc2YmYRmagcEPdU9",
		},
		{
			derivationPath: "m/0/2147483647H/1",
			expXpub:        "xpub6DF8uhdarytz3FWdA8TvFSvvAh8dP3283MY7p2V4SeE2wyWmG5mg5EwVvmdMVCQcoNJxGoWaU9DCWh89LojfZ537wTfunKau47EL2dhHKon",
			expXpriv:       "xprv9zFnWC6h2cLgpmSA46vutJzBcfJ8yaJGg8cX1e5StJh45BBciYTRXSd25UEPVuesF9yog62tGAQtHjXajPPdbRCHuWS6T8XA2ECKADdw4Ef",
		},
		{
			derivationPath: "m/0/2147483647H/1/2147483646H",
			expXpub:        "xpub6ERApfZwUNrhLCkDtcHTcxd75RbzS1ed54G1LkBUHQVHQKqhMkhgbmJbZRkrgZw4koxb5JaHWkY4ALHY2grBGRjaDMzQLcgJvLJuZZvRcEL",
			expXpriv:       "xprvA1RpRA33e1JQ7ifknakTFpgNXPmW2YvmhqLQYMmrj4xJXXWYpDPS3xz7iAxn8L39njGVyuoseXzU6rcxFLJ8HFsTjSyQbLYnMpCqE2VbFWc",
		},
		{
			derivationPath: "m/0/2147483647H/1/2147483646H/2",
			expXpub:        "xpub6FnCn6nSzZAw5Tw7cgR9bi15UV96gLZhjDstkXXxvCLsUXBGXPdSnLFbdpq8p9HmGsApME5hQTZ3emM2rnY5agb9rXpVGyy3bdW6EEgAtqt",
			expXpriv:       "xprvA2nrNbFZABcdryreWet9Ea4LvTJcGsqrMzxHx98MMrotbir7yrKCEXw7nadnHM8Dq38EGfSh6dqA9QWTyefMLEcBYJUuekgW4BYPJcr9E7j",
		},
	}
	derive := func(seed []byte, path string) (string, string, error) {
		return FromSeed(seed, path)
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			xpub, xpriv, err := derive(seed, test.derivationPath)
			assert.Must(t, err)
			mustEqualEncodedExt(t, test.expXpub, xpub)
			mustEqualEncodedExt(t, test.expXpriv, xpriv)
		})
	}
}

func TestBip32_vector3(t *testing.T) {
	seed := testutil.MustHexDecode("4b381541583be4423346c643850da4b320e46a87ae3d2a4e6da11eba819cd4acba45d239319ac14f863b8d5ab5a0d0c64d2e8a1e7d1457df2e5a3c51c73235be")
	tests := []struct {
		derivationPath string
		expXpub        string
		expXpriv       string
	}{
		{
			derivationPath: "m",
			expXpub:        "xpub661MyMwAqRbcEZVB4dScxMAdx6d4nFc9nvyvH3v4gJL378CSRZiYmhRoP7mBy6gSPSCYk6SzXPTf3ND1cZAceL7SfJ1Z3GC8vBgp2epUt13",
			expXpriv:       "xprv9s21ZrQH143K25QhxbucbDDuQ4naNntJRi4KUfWT7xo4EKsHt2QJDu7KXp1A3u7Bi1j8ph3EGsZ9Xvz9dGuVrtHHs7pXeTzjuxBrCmmhgC6",
		},
		{
			derivationPath: "m/0H",
			expXpub:        "xpub68NZiKmJWnxxS6aaHmn81bvJeTESw724CRDs6HbuccFQN9Ku14VQrADWgqbhhTHBaohPX4CjNLf9fq9MYo6oDaPPLPxSb7gwQN3ih19Zm4Y",
			expXpriv:       "xprv9uPDJpEQgRQfDcW7BkF7eTya6RPxXeJCqCJGHuCJ4GiRVLzkTXBAJMu2qaMWPrS7AANYqdq6vcBcBUdJCVVFceUvJFjaPdGZ2y9WACViL4L",
		},
	}
	derive := func(seed []byte, path string) (string, string, error) {
		return FromSeed(seed, path)
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			xpub, xpriv, err := derive(seed, test.derivationPath)
			assert.Must(t, err)
			mustEqualEncodedExt(t, test.expXpub, xpub)
			mustEqualEncodedExt(t, test.expXpriv, xpriv)
		})
	}
}

func TestBip32_vector4(t *testing.T) {
	seed := testutil.MustHexDecode("3ddd5602285899a946114506157c7997e5444528f3003f6134712147db19b678")
	tests := []struct {
		derivationPath string
		expXpub        string
		expXpriv       string
	}{
		{
			derivationPath: "m",
			expXpub:        "xpub661MyMwAqRbcGczjuMoRm6dXaLDEhW1u34gKenbeYqAix21mdUKJyuyu5F1rzYGVxyL6tmgBUAEPrEz92mBXjByMRiJdba9wpnN37RLLAXa",
			expXpriv:       "xprv9s21ZrQH143K48vGoLGRPxgo2JNkJ3J3fqkirQC2zVdk5Dgd5w14S7fRDyHH4dWNHUgkvsvNDCkvAwcSHNAQwhwgNMgZhLtQC63zxwhQmRv",
		},
		{
			derivationPath: "m/0H",
			expXpub:        "xpub69AUMk3qDBi3uW1sXgjCmVjJ2G6WQoYSnNHyzkmdCHEhSZ4tBok37xfFEqHd2AddP56Tqp4o56AePAgCjYdvpW2PU2jbUPFKsav5ut6Ch1m",
			expXpriv:       "xprv9vB7xEWwNp9kh1wQRfCCQMnZUEG21LpbR9NPCNN1dwhiZkjjeGRnaALmPXCX7SgjFTiCTT6bXes17boXtjq3xLpcDjzEuGLQBM5ohqkao9G",
		},
		{
			derivationPath: "m/0H/1H",
			expXpub:        "xpub6BJA1jSqiukeaesWfxe6sNK9CCGaujFFSJLomWHprUL9DePQ4JDkM5d88n49sMGJxrhpjazuXYWdMf17C9T5XnxkopaeS7jGk1GyyVziaMt",
			expXpriv:       "xprv9xJocDuwtYCMNAo3Zw76WENQeAS6WGXQ55RCy7tDJ8oALr4FWkuVoHJeHVAcAqiZLE7Je3vZJHxspZdFHfnBEjHqU5hG1Jaj32dVoS6XLT1",
		},
	}
	derive := func(seed []byte, path string) (string, string, error) {
		return FromSeed(seed, path)
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			xpub, xpriv, err := derive(seed, test.derivationPath)
			assert.Must(t, err)
			mustEqualEncodedExt(t, test.expXpub, xpub)
			mustEqualEncodedExt(t, test.expXpriv, xpriv)
		})
	}
}

func TestBip32_vector5(t *testing.T) {
	tests := []struct {
		xKey string
		err  error
	}{
		{
			xKey: "xpub661MyMwAqRbcEYS8w7XLSVeEsBXy79zSzH1J8vCdxAZningWLdN3zgtU6LBpB85b3D2yc8sfvZU521AAwdZafEz7mnzBBsz4wKY5fTtTQBm",
			err:  errBadKeyVersion,
		},
		{
			xKey: "xprv9s21ZrQH143K24Mfq5zL5MhWK9hUhhGbd45hLXo2Pq2oqzMMo63oStZzFGTQQD3dC4H2D5GBj7vWvSQaaBv5cxi9gafk7NF3pnBju6dwKvH",
			err:  errBadKeyVersion,
		},
		{
			xKey: "xpub661MyMwAqRbcEYS8w7XLSVeEsBXy79zSzH1J8vCdxAZningWLdN3zgtU6Txnt3siSujt9RCVYsx4qHZGc62TG4McvMGcAUjeuwZdduYEvFn",
			err:  errBadPubKeyPrefix,
		},
		{
			xKey: "xprv9s21ZrQH143K24Mfq5zL5MhWK9hUhhGbd45hLXo2Pq2oqzMMo63oStZzFGpWnsj83BHtEy5Zt8CcDr1UiRXuWCmTQLxEK9vbz5gPstX92JQ",
			err:  errBadPrivKeyPrefix,
		},
		{
			xKey: "xpub661MyMwAqRbcEYS8w7XLSVeEsBXy79zSzH1J8vCdxAZningWLdN3zgtU6N8ZMMXctdiCjxTNq964yKkwrkBJJwpzZS4HS2fxvyYUA4q2Xe4",
			err:  errBadPubKeyPrefix,
		},
		{
			xKey: "xprv9s21ZrQH143K24Mfq5zL5MhWK9hUhhGbd45hLXo2Pq2oqzMMo63oStZzFAzHGBP2UuGCqWLTAPLcMtD9y5gkZ6Eq3Rjuahrv17fEQ3Qen6J",
			err:  errBadPrivKeyPrefix,
		},
		{
			xKey: "xprv9s2SPatNQ9Vc6GTbVMFPFo7jsaZySyzk7L8n2uqKXJen3KUmvQNTuLh3fhZMBoG3G4ZW1N2kZuHEPY53qmbZzCHshoQnNf4GvELZfqTUrcv",
			err:  errZeroDepthNonZeroFingerprint,
		},
		{
			xKey: "xpub661no6RGEX3uJkY4bNnPcw4URcQTrSibUZ4NqJEw5eBkv7ovTwgiT91XX27VbEXGENhYRCf7hyEbWrR3FewATdCEebj6znwMfQkhRYHRLpJ",
			err:  errZeroDepthNonZeroFingerprint,
		},
		{
			xKey: "xprv9s21ZrQH4r4TsiLvyLXqM9P7k1K3EYhA1kkD6xuquB5i39AU8KF42acDyL3qsDbU9NmZn6MsGSUYZEsuoePmjzsB3eFKSUEh3Gu1N3cqVUN",
			err:  errZeroDepthNonZeroIndex,
		},
		{
			xKey: "xpub661MyMwAuDcm6CRQ5N4qiHKrJ39Xe1R1NyfouMKTTWcguwVcfrZJaNvhpebzGerh7gucBvzEQWRugZDuDXjNDRmXzSZe4c7mnTK97pTvGS8",
			err:  errZeroDepthNonZeroIndex,
		},
		{
			xKey: "DMwo58pR1QLEFihHiXPVykYB6fJmsTeHvyTp7hRThAtCX8CvYzgPcn8XnmdfHGMQzT7ayAmfo4z3gY5KfbrZWZ6St24UVf2Qgo6oujFktLHdHY4",
			err:  errUnknownVersion,
		},
		{
			xKey: "DMwo58pR1QLEFihHiXPVykYB6fJmsTeHvyTp7hRThAtCX8CvYzgPcn8XnmdfHPmHJiEDXkTiJTVV9rHEBUem2mwVbbNfvT2MTcAqj3nesx8uBf9",
			err:  errUnknownVersion,
		},
		{
			xKey: "xprv9s21ZrQH143K24Mfq5zL5MhWK9hUhhGbd45hLXo2Pq2oqzMMo63oStZzF93Y5wvzdUayhgkkFoicQZcP3y52uPPxFnfoLZB21Teqt1VvEHx",
			err:  errPrivKeyNotInCurveOrder,
		},
		{
			xKey: "xprv9s21ZrQH143K24Mfq5zL5MhWK9hUhhGbd45hLXo2Pq2oqzMMo63oStZzFAzHGBP2UuGCqWLTAPLcMtD5SDKr24z3aiUvKr9bJpdrcLg1y3G",
			err:  errPrivKeyNotInCurveOrder,
		},
		{
			xKey: "xpub661MyMwAqRbcEYS8w7XLSVeEsBXy79zSzH1J8vCdxAZningWLdN3zgtU6Q5JXayek4PRsn35jii4veMimro1xefsM58PgBMrvdYre8QyULY",
			err:  errInvalidPubKey,
		},
		{
			xKey: "xprv9s21ZrQH143K3QTDL4LXw2F7HEK3wJUD2nW2nRk4stbPy6cq3jPPqjiChkVvvNKmPGJxWUtg6LnF5kejMRNNU3TGtRBeJgk33yuGBxrMPHL",
			err:  errInvalidChecksum,
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			_, err := extendedDecode(test.xKey)
			assert.MustEqual(t, test.err, err)
		})
	}
}

func Test_ParseDerivationPath(t *testing.T) {
	tests := []struct {
		path string
		exp  []uint32
	}{
		{
			path: "m/0H/1/2H/2/1000000000",
			exp:  []uint32{0 | hardened, 1, 2 | hardened, 2, 1000000000},
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			path, err := parseDerivationPath(test.path)
			assert.Must(t, err)
			assert.MustEqual(t, test.exp, path)
		})
	}
}

func Test_DecodeExtendedKey(t *testing.T) {
	tests := []struct {
		xkey string
		exp  extendedKey
	}{
		{
			xkey: "xprvA41z7zogVVwxVSgdKUHDy1SKmdb533PjDz7J6N6mV6uS3ze1ai8FHa8kmHScGpWmj4WggLyQjgPie1rFSruoUihUZREPSL39UNdE3BBDu76",
			exp: extendedKey{
				Version: versionMainnetPrivate,
				Depth:   0x5,
				Fingerprint: [4]byte(testutil.MustHexDecode(
					"d880d7d8",
				)),
				ChildNum: [4]byte(testutil.MustHexDecode(
					"3b9aca00",
				)),
				Chaincode: [32]byte(testutil.MustHexDecode(
					"c783e67b921d2beb8f6b389cc646d7263b4145701dadd2161548a8b078e65e9e",
				)),
				Key: [33]byte(testutil.MustHexDecode(
					"00471b76e389e528d6de6d816857e012c5455051cad6660850e58372a6c3e6e7c8",
				)),
			},
		},
		{
			xkey: "xpub6H1LXWLaKsWFhvm6RVpEL9P4KfRZSW7abD2ttkWP3SSQvnyA8FSVqNTEcYFgJS2UaFcxupHiYkro49S8yGasTvXEYBVPamhGW6cFJodrTHy",
			exp: extendedKey{
				Version: versionMainnetPublic,
				Depth:   0x5,
				Fingerprint: [4]byte(testutil.MustHexDecode(
					"d880d7d8",
				)),
				ChildNum: [4]byte(testutil.MustHexDecode(
					"3b9aca00",
				)),
				Chaincode: [32]byte(testutil.MustHexDecode(
					"c783e67b921d2beb8f6b389cc646d7263b4145701dadd2161548a8b078e65e9e",
				)),
				Key: [33]byte(testutil.MustHexDecode(
					"022a471424da5e657499d1ff51cb43c47481a03b1e77f951fe64cec9f5a48f7011",
				)),
			},
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			decoded, err := extendedDecode(test.xkey)
			assert.Must(t, err)
			mustEqualExt(t, test.exp, decoded)
		})
	}
}

func mustEqualExt(t *testing.T, exp, got extendedKey) {
	if reflect.DeepEqual(exp, got) {
		return
	}
	t.Fatalf(
		"\nexp: %x %x %x %x %x %x\ngot: %x %x %x %x %x %x\n",
		exp.Version,
		exp.Depth,
		exp.Fingerprint,
		exp.ChildNum,
		exp.Chaincode,
		exp.Key,
		got.Version,
		got.Depth,
		got.Fingerprint,
		got.ChildNum,
		got.Chaincode,
		got.Key,
	)
}

func mustEqualEncodedExt(t *testing.T, expS, gotS string) {
	if reflect.DeepEqual(expS, gotS) {
		return
	}
	exp, err := extendedDecode(expS)
	assert.Must(t, err)
	got, err := extendedDecode(gotS)
	assert.Must(t, err)
	t.Fatalf(
		"\nexp: %x %x %x %x %x %x\ngot: %x %x %x %x %x %x\n",
		exp.Version,
		exp.Depth,
		exp.Fingerprint,
		exp.ChildNum,
		exp.Chaincode,
		exp.Key,
		got.Version,
		got.Depth,
		got.Fingerprint,
		got.ChildNum,
		got.Chaincode,
		got.Key,
	)
}
