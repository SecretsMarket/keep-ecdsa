package cmd

import (
	"fmt"

	"github.com/keep-network/keep-tecdsa/pkg/sign"
	"github.com/urfave/cli"
)

// SignCommand contains the definition of the sign command-line subcommand.
var SignCommand cli.Command

const signDescription = `The sign command creates a local signer and calculates
   a signature over provided argument.`

func init() {
	SignCommand = cli.Command{
		Name:        "sign",
		Usage:       "Calculates a signature",
		Description: signDescription,
		Action:      Sign,
	}
}

// Sign creates a local signer, generates a public/private ECDSA key pair for them
// and calculates a signature over a provided CLI argument.
func Sign(c *cli.Context) error {
	arg := c.Args().First()

	privateKey,err := sign.GenerateKey()
	if err != nil {
		return fmt.Errorf("key generation failed [%v]", err)
	}

	signer, err := sign.NewSigner(privateKey)
	if err != nil {
		return fmt.Errorf("signer creation failed [%v]", err)
	}

	fmt.Printf("--- Generated Public Key:\nX: %x\nY: %x\n",
		signer.PublicKey().X,
		signer.PublicKey().Y,
	)

	signature, err := signer.CalculateSignature([]byte(arg))
	if err != nil {
		return fmt.Errorf("signature calculation failed [%v]", err)
	}

	fmt.Printf("--- Signature:\nR: %x\nS: %x\n", signature.R, signature.S)

	return nil
}
