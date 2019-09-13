// Package btc handles bitcoin transactions signing and publishing.
package btc

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/ipfs/go-log"
	"github.com/keep-network/keep-tecdsa/pkg/chain/btc"
	"github.com/keep-network/keep-tecdsa/pkg/sign"
	"github.com/keep-network/keep-tecdsa/pkg/utils"
)

var logger = log.Logger("keep-btc")

// SignAndPublishTransaction calculates signature over Witness Signature Hash for
// a transaction, sets the signature on the transaction and publishes it to the
// chain.
//
// Witness Signature Hash is expected to be calculated according to digest
// calculation algorithm defined in [BIP-143].
//
// Transaction preimage is a raw unsigned transaction. Only transaction with
// one input are supported. The first and only input is signed with witness.
//
// [BIP-143]: https://github.com/bitcoin/bips/blob/master/bip-0143.mediawiki#specification
func SignAndPublishTransaction(
	rand io.Reader,
	chain btc.Interface,
	signer *sign.Signer,
	witnessSignatureHash []byte,
	transactionPreimage []byte,
) error {
	transaction, err := utils.DeserializeTransaction(transactionPreimage)
	if err != nil {
		return fmt.Errorf("cannot deserialize transaction preimage [%s]", err)
	}

	if len(transaction.TxIn) > 1 {
		return fmt.Errorf("only transactions with one input are supported")
	}
	inputIndex := 0

	signature, err := signer.CalculateSignature(rand, witnessSignatureHash)
	if err != nil {
		return err
	}

	// TODO: Publish signature to the ethereum chain and add validation to test
	logger.Debugf("Signature: %v", signature)

	SetSignatureWitnessToTransaction(
		signature,
		signer.PublicKey(),
		inputIndex,
		transaction,
	)

	rawSignedTransaction, err := utils.SerializeTransaction(transaction)
	if err != nil {
		return err
	}

	logger.Debugf("Publish transaction:\n%v", hex.EncodeToString(rawSignedTransaction))

	transactionHash, err := Publish(chain, rawSignedTransaction)
	if err != nil {
		return fmt.Errorf("transaction publication failed [%s]", err)
	}

	logger.Debugf("Published transaction hash: %v", transactionHash)

	return nil
}
