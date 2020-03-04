// Package client defines ECDSA keep client.
package client

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-common/pkg/persistence"
	"github.com/keep-network/keep-core/pkg/net"
	"github.com/keep-network/keep-core/pkg/operator"
	"github.com/keep-network/keep-tecdsa/pkg/chain/eth"
	"github.com/keep-network/keep-tecdsa/pkg/ecdsa/tss"
	"github.com/keep-network/keep-tecdsa/pkg/node"
	"github.com/keep-network/keep-tecdsa/pkg/registry"
	"github.com/keep-network/keep-common/pkg/subscription"
)

var logger = log.Logger("keep-tecdsa")

// Initialize initializes the ECDSA client with rules related to events handling.
// Expects a slice of sanctioned applications selected by the operator for which
// operator will be registered as a member candidate.
func Initialize(
	ctx context.Context,
	operatorPublicKey *operator.PublicKey,
	ethereumChain eth.Handle,
	networkProvider net.Provider,
	persistence persistence.Handle,
	sanctionedApplications []common.Address,
) {
	keepsRegistry := registry.NewKeepsRegistry(persistence)

	tssNode := node.NewNode(ethereumChain, networkProvider)

	tssNode.InitializeTSSPreParamsPool()

	// Load current keeps' signers from storage and register for signing events.
	keepsRegistry.LoadExistingKeeps()

	keepsRegistry.ForEachKeep(
		func(keepAddress common.Address, signer []*tss.ThresholdSigner) {
			for _, signer := range signer {
				isActive, err := ethereumChain.IsActive(keepAddress)
				if err != nil {
					logger.Errorf(
						"failed to verify if keep is still active: [%v]. " + 
						"Subscriptions for keep signing and closing events are skipped", 
						err,
					)
					continue
				}

				if isActive {
					subscriptionOnSignatureRequested := monitorSignatureEvents(
						ethereumChain,
						tssNode,
						keepAddress,
						signer,
					)
					go monitorKeepClosedEvents(
						ethereumChain,
						keepAddress,
						keepsRegistry,
						subscriptionOnSignatureRequested,
					)
					logger.Debugf(
						"signer registered for events from keep: [%s]",
						keepAddress.String(),
					)
				} else {
					logger.Infof(
						"keep [%s] is no longer active; archiving",
						keepAddress.String(),
					)
					keepsRegistry.UnregisterKeep(keepAddress)
				}
			}
		},
	)

	// Watch for new keeps creation.
	ethereumChain.OnBondedECDSAKeepCreated(func(event *eth.BondedECDSAKeepCreatedEvent) {
		logger.Infof(
			"new keep [%s] created with members: [%x]\n",
			event.KeepAddress.String(),
			event.Members,
		)

		if event.IsMember(ethereumChain.Address()) {
			logger.Infof(
				"member [%s] is starting signer generation for keep [%s]...",
				ethereumChain.Address().String(),
				event.KeepAddress.String(),
			)

			memberIDs, err := tssNode.AnnounceSignerPresence(
				operatorPublicKey,
				event.KeepAddress,
				event.Members,
			)

			signer, err := tssNode.GenerateSignerForKeep(
				operatorPublicKey,
				event.KeepAddress,
				memberIDs,
			)
			if err != nil {
				logger.Errorf("signer generation failed: [%v]", err)
				return
			}

			logger.Infof("initialized signer for keep [%s]", event.KeepAddress.String())

			err = keepsRegistry.RegisterSigner(event.KeepAddress, signer)
			if err != nil {
				logger.Errorf(
					"failed to register threshold signer for keep [%s]: [%v]",
					event.KeepAddress.String(),
					err,
				)
			}

			subscriptionOnSignatureRequested := monitorSignatureEvents(
				ethereumChain,
				tssNode,
				event.KeepAddress,
				signer,
			)

			go monitorKeepClosedEvents(
				ethereumChain,
				event.KeepAddress,
				keepsRegistry,
				subscriptionOnSignatureRequested,
			)
		}
	})

	for _, application := range sanctionedApplications {
		go checkStatusAndRegisterForApplication(ctx, ethereumChain, application)
	}
}

// monitorSignatureEvents registers for signature requested events emitted by
// specific keep contract.
func monitorSignatureEvents(
	ethereumChain eth.Handle,
	tssNode *node.Node,
	keepAddress common.Address,
	signer *tss.ThresholdSigner,
) (subscription.EventSubscription) {
	subscriptionOnSignatureRequested, err := ethereumChain.OnSignatureRequested(
		keepAddress,
		func(signatureRequestedEvent *eth.SignatureRequestedEvent) {
			logger.Infof(
				"new signature requested from keep [%s] for digest: [%+x]",
				keepAddress.String(),
				signatureRequestedEvent.Digest,
			)

			go func() {
				err := tssNode.CalculateSignature(
					signer,
					signatureRequestedEvent.Digest,
				)

				if err != nil {
					logger.Errorf("signature calculation failed: [%v]", err)
				}
			}()
		},
	)
	if err != nil {
		logger.Errorf(
			"failed on registering for requested signature event: [%v]",
			err,
		)
	}

	return subscriptionOnSignatureRequested
}

// monitorKeepClosedEvents subscribes and unsubscribes for keep closed events.
func monitorKeepClosedEvents(
	ethereumChain eth.Handle,
	keepAddress common.Address,
	keepsRegistry *registry.Keeps,
	subscriptionOnSignatureRequested subscription.EventSubscription,
) {
	keepClosed := make(chan *eth.KeepClosedEvent)

	subscriptionOnKeepClosed, err := ethereumChain.OnKeepClosed(
		keepAddress,
		func(event *eth.KeepClosedEvent) {
			logger.Infof("keep [%s] is being closed", keepAddress.String())
			keepsRegistry.UnregisterKeep(keepAddress)
			keepClosed <- event
		},
	)
	if err != nil {
		logger.Errorf(
			"failed on registering for keep closed event: [%v]",
			err,
		)
	}

	defer subscriptionOnKeepClosed.Unsubscribe()
	defer subscriptionOnSignatureRequested.Unsubscribe()

	<- keepClosed

	logger.Info(
		"unsubscribing from events on keep closure",
	)

}
