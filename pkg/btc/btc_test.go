package btc

import (
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/btcsuite/btcutil"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/keep-network/keep-tecdsa/internal/testdata"
	"github.com/keep-network/keep-tecdsa/pkg/chain/btc/local"
	"github.com/keep-network/keep-tecdsa/pkg/sign"
)

func TestSignAndPublishTransaction(t *testing.T) {
	witnessSignatureHash, _ := hex.DecodeString(testdata.ValidTx.WitnessSignatureHash)
	transactionPreimage, _ := hex.DecodeString(testdata.ValidTx.UnsignedRaw)

	signer, err := newTestSigner()
	if err != nil {
		t.Fatal(err)
	}

	chain := local.Connect()
	err = SignAndPublishTransaction(crand.Reader, chain, signer, witnessSignatureHash, transactionPreimage)
	if err != nil {
		t.Error(err)
	}
}

func newTestSigner() (*sign.Signer, error) {
	curve := secp256k1.S256()

	wif, err := btcutil.DecodeWIF("923CjseKgQf7Xz185dmYUJer9i8jsb9Cd18Rtec4DFKeiBZg3wi")
	if err != nil {
		return nil, fmt.Errorf("cannot decode WIF [%s]", err)
	}

	k := wif.PrivKey.D

	privateKey := new(ecdsa.PrivateKey)
	privateKey.PublicKey.Curve = curve
	privateKey.D = k
	privateKey.PublicKey.X, privateKey.PublicKey.Y = curve.ScalarBaseMult(k.Bytes())

	return sign.NewSigner(privateKey), nil
}
