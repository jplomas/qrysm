package validator

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	dilithium2 "github.com/theQRL/go-qrllib/dilithium"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	"github.com/theQRL/qrysm/v4/consensus-types/primitives"
	"github.com/theQRL/qrysm/v4/crypto/dilithium"
	zondpb "github.com/theQRL/qrysm/v4/proto/prysm/v1alpha1"
	synccontribution "github.com/theQRL/qrysm/v4/proto/prysm/v1alpha1/attestation/aggregation/sync_contribution"
	"github.com/theQRL/qrysm/v4/runtime/version"
	"go.opencensus.io/trace"
)

func (vs *Server) setSyncAggregate(ctx context.Context, blk interfaces.SignedBeaconBlock) {
	if blk.Version() < version.Altair {
		return
	}

	syncAggregate, err := vs.getSyncAggregate(ctx, blk.Block().Slot()-1, blk.Block().ParentRoot())
	if err != nil {
		log.WithError(err).Error("Could not get sync aggregate")
		emptySig := [dilithium2.CryptoBytes]byte{0xC0}
		emptyAggregate := &zondpb.SyncAggregate{
			SyncCommitteeBits:      make([]byte, params.BeaconConfig().SyncCommitteeSize/8),
			SyncCommitteeSignature: emptySig[:],
		}
		if err := blk.SetSyncAggregate(emptyAggregate); err != nil {
			log.WithError(err).Error("Could not set sync aggregate")
		}
		return
	}

	// Can not error. We already filter block versioning at the top. Phase 0 is impossible.
	if err := blk.SetSyncAggregate(syncAggregate); err != nil {
		log.WithError(err).Error("Could not set sync aggregate")
	}
}

// getSyncAggregate retrieves the sync contributions from the pool to construct the sync aggregate object.
// The contributions are filtered based on matching of the input root and slot then profitability.
func (vs *Server) getSyncAggregate(ctx context.Context, slot primitives.Slot, root [32]byte) (*zondpb.SyncAggregate, error) {
	ctx, span := trace.StartSpan(ctx, "ProposerServer.getSyncAggregate")
	defer span.End()

	if vs.SyncCommitteePool == nil {
		return nil, errors.New("sync committee pool is nil")
	}
	// Contributions have to match the input root
	contributions, err := vs.SyncCommitteePool.SyncCommitteeContributions(slot)
	if err != nil {
		return nil, err
	}
	proposerContributions := proposerSyncContributions(contributions).filterByBlockRoot(root)

	// Each sync subcommittee is 128 bits and the sync committee is 512 bits for mainnet.
	var bitsHolder [][]byte
	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
		bitsHolder = append(bitsHolder, zondpb.NewSyncCommitteeAggregationBits())
	}
	sigsHolder := make([]dilithium.Signature, 0, params.BeaconConfig().SyncCommitteeSize/params.BeaconConfig().SyncCommitteeSubnetCount)

	for i := uint64(0); i < params.BeaconConfig().SyncCommitteeSubnetCount; i++ {
		cs := proposerContributions.filterBySubIndex(i)
		aggregates, err := synccontribution.Aggregate(cs)
		if err != nil {
			return nil, err
		}

		// Retrieve the most profitable contribution
		deduped, err := proposerSyncContributions(aggregates).dedup()
		if err != nil {
			return nil, err
		}
		c := deduped.mostProfitable()
		if c == nil {
			continue
		}
		bitsHolder[i] = c.AggregationBits
		if len(c.Signature)%dilithium2.CryptoBytes != 0 {
			return nil, fmt.Errorf(
				"combined Signature length is %d is not in the multiple of %d",
				len(c.Signature), dilithium2.CryptoBytes)
		}
		for i := 0; i < len(c.Signature)/dilithium2.CryptoBytes; i++ {
			offset := i * dilithium2.CryptoBytes
			signature := c.Signature[offset : offset+dilithium2.CryptoBytes]
			sig, err := dilithium.SignatureFromBytes(signature)
			if err != nil {
				return nil, err
			}
			sigsHolder = append(sigsHolder, sig)
		}
	}

	// Aggregate all the contribution bits and signatures.
	var syncBits []byte
	for _, b := range bitsHolder {
		syncBits = append(syncBits, b...)
	}
	syncSig := dilithium.UnaggregatedSignatures(sigsHolder)
	var syncSigBytes []byte
	if syncSig == nil {
		var infSig = [dilithium2.CryptoBytes]byte{0xC0} // Infinity signature if itself is nil.
		syncSigBytes = infSig[:]
	} else {
		syncSigBytes = syncSig
	}

	return &zondpb.SyncAggregate{
		SyncCommitteeBits:      syncBits,
		SyncCommitteeSignature: syncSigBytes,
	}, nil
}
