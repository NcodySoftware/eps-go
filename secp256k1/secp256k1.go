package secp256k1

import (
	"github.com/ncodysoftware/eps-go/external/dcrd/secp256k1"
	"github.com/ncodysoftware/eps-go/external/dcrd/secp256k1/ecdsa"
)

type ModNScalar struct {
	s secp256k1.ModNScalar
}

func ModNScalarFromSlice(data []byte) ModNScalar {
	var s secp256k1.ModNScalar
	s.SetByteSlice(data)
	return ModNScalar{s}
}

func ModNScalarAdd(a, b *ModNScalar) ModNScalar {
	var r secp256k1.ModNScalar
	r.Add2(&a.s, &b.s)
	return ModNScalar{r}
}

func ModNScalarBytes(s *ModNScalar) [32]byte {
	return s.s.Bytes()
}

func ModNScalarBaseMult(s *ModNScalar) Point {
	var p secp256k1.JacobianPoint
	secp256k1.ScalarBaseMultNonConst(&s.s, &p)
	return Point{p}
}

type Point struct {
	p secp256k1.JacobianPoint
}

func PointDeserialize(data []byte) (Point, error) {
	var jp secp256k1.JacobianPoint
	p, err := secp256k1.ParsePubKey(data)
	if err != nil {
		return Point{}, err
	}
	p.AsJacobian(&jp)
	return Point{jp}, nil
}

func PointSerializeCompressed(p *Point) [33]byte {
	pub := secp256k1.NewPublicKey(&p.p.X, &p.p.Y)
	return [33]byte(pub.SerializeCompressed())
}

func PointAdd(a, b *Point) Point {
	var result secp256k1.JacobianPoint
	secp256k1.AddNonConst(&a.p, &b.p, &result)
	return Point{result}
}

func PointAtInfinity(p *Point) bool {
	return p.p.X.IsZero() && p.p.Y.IsZero() || p.p.Z.IsZero()
}

func PointToAffine(p *Point) {
	p.p.ToAffine()
}

func Sign(privateKey []byte, data []byte) []byte {
	priv := secp256k1.PrivKeyFromBytes(privateKey)
	return ecdsa.Sign(priv, data).Serialize()
}
