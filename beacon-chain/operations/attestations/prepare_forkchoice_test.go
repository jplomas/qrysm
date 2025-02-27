package attestations

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/theQRL/go-bitfield"
	"github.com/theQRL/qrysm/v4/config/features"
	"github.com/theQRL/qrysm/v4/crypto/bls"
	zondpb "github.com/theQRL/qrysm/v4/proto/prysm/v1alpha1"
	attaggregation "github.com/theQRL/qrysm/v4/proto/prysm/v1alpha1/attestation/aggregation/attestations"
	"github.com/theQRL/qrysm/v4/testing/assert"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/util"
	"google.golang.org/protobuf/proto"
)

func TestBatchAttestations_Multiple(t *testing.T) {
	resetFn := features.InitWithReset(&features.Flags{
		AggregateParallel: true,
	})
	defer resetFn()

	s, err := NewService(context.Background(), &Config{Pool: NewPool()})
	require.NoError(t, err)

	priv, err := bls.RandKey()
	require.NoError(t, err)
	sig := priv.Sign([]byte("dummy_test_data"))
	var mockRoot [32]byte

	unaggregatedAtts := []*zondpb.Attestation{
		{Data: &zondpb.AttestationData{
			Slot:            2,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b100100}, Signature: sig.Marshal()},
		{Data: &zondpb.AttestationData{
			Slot:            1,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b101000}, Signature: sig.Marshal()},
		{Data: &zondpb.AttestationData{
			Slot:            0,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b100010}, Signature: sig.Marshal()},
	}
	aggregatedAtts := []*zondpb.Attestation{
		{Data: &zondpb.AttestationData{
			Slot:            2,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b111000}, Signature: sig.Marshal()},
		{Data: &zondpb.AttestationData{
			Slot:            1,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b100011}, Signature: sig.Marshal()},
		{Data: &zondpb.AttestationData{
			Slot:            0,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b110001}, Signature: sig.Marshal()},
	}
	blockAtts := []*zondpb.Attestation{
		{Data: &zondpb.AttestationData{
			Slot:            2,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b100001}, Signature: sig.Marshal()},
		{Data: &zondpb.AttestationData{
			Slot:            1,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b100100}, Signature: sig.Marshal()},
		{Data: &zondpb.AttestationData{
			Slot:            0,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b100100}, Signature: sig.Marshal()},
		{Data: &zondpb.AttestationData{
			Slot:            2,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b111000}, Signature: sig.Marshal()}, // Duplicated
		{Data: &zondpb.AttestationData{
			Slot:            1,
			BeaconBlockRoot: mockRoot[:],
			Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
			Target:          &zondpb.Checkpoint{Root: mockRoot[:]}}, AggregationBits: bitfield.Bitlist{0b100011}, Signature: sig.Marshal()}, // Duplicated
	}
	require.NoError(t, s.cfg.Pool.SaveUnaggregatedAttestations(unaggregatedAtts))
	require.NoError(t, s.cfg.Pool.SaveAggregatedAttestations(aggregatedAtts))
	for _, att := range blockAtts {
		require.NoError(t, s.cfg.Pool.SaveBlockAttestation(att))
	}
	require.NoError(t, s.batchForkChoiceAtts(context.Background()))

	wanted, err := attaggregation.Aggregate([]*zondpb.Attestation{aggregatedAtts[0], blockAtts[0]})
	require.NoError(t, err)
	aggregated, err := attaggregation.Aggregate([]*zondpb.Attestation{aggregatedAtts[1], blockAtts[1]})
	require.NoError(t, err)
	wanted = append(wanted, aggregated...)
	aggregated, err = attaggregation.Aggregate([]*zondpb.Attestation{aggregatedAtts[2], blockAtts[2]})
	require.NoError(t, err)

	wanted = append(wanted, aggregated...)
	require.NoError(t, s.cfg.Pool.AggregateUnaggregatedAttestations(context.Background()))
	received := s.cfg.Pool.ForkchoiceAttestations()

	sort.Slice(received, func(i, j int) bool {
		return received[i].Data.Slot < received[j].Data.Slot
	})
	sort.Slice(wanted, func(i, j int) bool {
		return wanted[i].Data.Slot < wanted[j].Data.Slot
	})

	assert.DeepSSZEqual(t, wanted, received)
}

func TestBatchAttestations_Single(t *testing.T) {
	s, err := NewService(context.Background(), &Config{Pool: NewPool()})
	require.NoError(t, err)

	priv, err := bls.RandKey()
	require.NoError(t, err)
	sig := priv.Sign([]byte("dummy_test_data"))
	var mockRoot [32]byte
	d := &zondpb.AttestationData{
		BeaconBlockRoot: mockRoot[:],
		Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
		Target:          &zondpb.Checkpoint{Root: mockRoot[:]},
	}

	unaggregatedAtts := []*zondpb.Attestation{
		{Data: d, AggregationBits: bitfield.Bitlist{0b101000}, Signature: sig.Marshal()},
		{Data: d, AggregationBits: bitfield.Bitlist{0b100100}, Signature: sig.Marshal()},
	}
	aggregatedAtts := []*zondpb.Attestation{
		{Data: d, AggregationBits: bitfield.Bitlist{0b101100}, Signature: sig.Marshal()},
		{Data: d, AggregationBits: bitfield.Bitlist{0b110010}, Signature: sig.Marshal()},
	}
	blockAtts := []*zondpb.Attestation{
		{Data: d, AggregationBits: bitfield.Bitlist{0b110010}, Signature: sig.Marshal()},
		{Data: d, AggregationBits: bitfield.Bitlist{0b100010}, Signature: sig.Marshal()},
		{Data: d, AggregationBits: bitfield.Bitlist{0b110010}, Signature: sig.Marshal()}, // Duplicated
	}
	require.NoError(t, s.cfg.Pool.SaveUnaggregatedAttestations(unaggregatedAtts))
	require.NoError(t, s.cfg.Pool.SaveAggregatedAttestations(aggregatedAtts))

	for _, att := range blockAtts {
		require.NoError(t, s.cfg.Pool.SaveBlockAttestation(att))
	}
	require.NoError(t, s.batchForkChoiceAtts(context.Background()))

	wanted, err := attaggregation.Aggregate(append(aggregatedAtts, unaggregatedAtts...))
	require.NoError(t, err)

	wanted, err = attaggregation.Aggregate(append(wanted, blockAtts...))
	require.NoError(t, err)

	got := s.cfg.Pool.ForkchoiceAttestations()
	assert.DeepEqual(t, wanted, got)
}

func TestAggregateAndSaveForkChoiceAtts_Single(t *testing.T) {
	s, err := NewService(context.Background(), &Config{Pool: NewPool()})
	require.NoError(t, err)

	priv, err := bls.RandKey()
	require.NoError(t, err)
	sig := priv.Sign([]byte("dummy_test_data"))
	var mockRoot [32]byte
	d := &zondpb.AttestationData{
		BeaconBlockRoot: mockRoot[:],
		Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
		Target:          &zondpb.Checkpoint{Root: mockRoot[:]},
	}

	atts := []*zondpb.Attestation{
		{Data: d, AggregationBits: bitfield.Bitlist{0b101}, Signature: sig.Marshal()},
		{Data: d, AggregationBits: bitfield.Bitlist{0b110}, Signature: sig.Marshal()}}
	require.NoError(t, s.aggregateAndSaveForkChoiceAtts(atts))

	wanted, err := attaggregation.Aggregate(atts)
	require.NoError(t, err)
	assert.DeepEqual(t, wanted, s.cfg.Pool.ForkchoiceAttestations())
}

func TestAggregateAndSaveForkChoiceAtts_Multiple(t *testing.T) {
	s, err := NewService(context.Background(), &Config{Pool: NewPool()})
	require.NoError(t, err)

	priv, err := bls.RandKey()
	require.NoError(t, err)
	sig := priv.Sign([]byte("dummy_test_data"))
	var mockRoot [32]byte
	d := &zondpb.AttestationData{
		BeaconBlockRoot: mockRoot[:],
		Source:          &zondpb.Checkpoint{Root: mockRoot[:]},
		Target:          &zondpb.Checkpoint{Root: mockRoot[:]},
	}
	d1, ok := proto.Clone(d).(*zondpb.AttestationData)
	require.Equal(t, true, ok, "Entity is not of type *zondpb.AttestationData")
	d1.Slot = 1
	d2, ok := proto.Clone(d).(*zondpb.AttestationData)
	require.Equal(t, true, ok, "Entity is not of type *zondpb.AttestationData")
	d2.Slot = 2

	atts1 := []*zondpb.Attestation{
		{Data: d, AggregationBits: bitfield.Bitlist{0b101}, Signature: sig.Marshal()},
		{Data: d, AggregationBits: bitfield.Bitlist{0b110}, Signature: sig.Marshal()},
	}
	require.NoError(t, s.aggregateAndSaveForkChoiceAtts(atts1))
	atts2 := []*zondpb.Attestation{
		{Data: d1, AggregationBits: bitfield.Bitlist{0b10110}, Signature: sig.Marshal()},
		{Data: d1, AggregationBits: bitfield.Bitlist{0b11100}, Signature: sig.Marshal()},
		{Data: d1, AggregationBits: bitfield.Bitlist{0b11000}, Signature: sig.Marshal()},
	}
	require.NoError(t, s.aggregateAndSaveForkChoiceAtts(atts2))
	att3 := []*zondpb.Attestation{
		{Data: d2, AggregationBits: bitfield.Bitlist{0b1100}, Signature: sig.Marshal()},
	}
	require.NoError(t, s.aggregateAndSaveForkChoiceAtts(att3))

	wanted, err := attaggregation.Aggregate(atts1)
	require.NoError(t, err)
	aggregated, err := attaggregation.Aggregate(atts2)
	require.NoError(t, err)

	wanted = append(wanted, aggregated...)
	wanted = append(wanted, att3...)

	received := s.cfg.Pool.ForkchoiceAttestations()
	sort.Slice(received, func(i, j int) bool {
		return received[i].Data.Slot < received[j].Data.Slot
	})
	for i, a := range wanted {
		assert.Equal(t, true, proto.Equal(a, received[i]))
	}
}

func TestSeenAttestations_PresentInCache(t *testing.T) {
	s, err := NewService(context.Background(), &Config{Pool: NewPool()})
	require.NoError(t, err)

	ad1 := util.HydrateAttestationData(&zondpb.AttestationData{})
	att1 := &zondpb.Attestation{Data: ad1, Signature: []byte{'A'}, AggregationBits: bitfield.Bitlist{0x13} /* 0b00010011 */}
	got, err := s.seen(att1)
	require.NoError(t, err)
	assert.Equal(t, false, got)

	att2 := &zondpb.Attestation{Data: ad1, Signature: []byte{'A'}, AggregationBits: bitfield.Bitlist{0x17} /* 0b00010111 */}
	got, err = s.seen(att2)
	require.NoError(t, err)
	assert.Equal(t, false, got)

	att3 := &zondpb.Attestation{Data: ad1, Signature: []byte{'A'}, AggregationBits: bitfield.Bitlist{0x17} /* 0b00010111 */}
	got, err = s.seen(att3)
	require.NoError(t, err)
	assert.Equal(t, true, got)
}

func TestService_seen(t *testing.T) {
	ad1 := util.HydrateAttestationData(&zondpb.AttestationData{Slot: 1})

	ad2 := util.HydrateAttestationData(&zondpb.AttestationData{Slot: 2})

	// Attestation are checked in order of this list.
	tests := []struct {
		att  *zondpb.Attestation
		want bool
	}{
		{
			att: &zondpb.Attestation{
				AggregationBits: bitfield.Bitlist{0b11011},
				Data:            ad1,
			},
			want: false,
		},
		{
			att: &zondpb.Attestation{
				AggregationBits: bitfield.Bitlist{0b11011},
				Data:            ad1,
			},
			want: true, // Exact same attestation should return true
		},
		{
			att: &zondpb.Attestation{
				AggregationBits: bitfield.Bitlist{0b10101},
				Data:            ad1,
			},
			want: false, // Haven't seen the bit at index 2 yet.
		},
		{
			att: &zondpb.Attestation{
				AggregationBits: bitfield.Bitlist{0b11111},
				Data:            ad1,
			},
			want: true, // We've full committee at this point.
		},
		{
			att: &zondpb.Attestation{
				AggregationBits: bitfield.Bitlist{0b11111},
				Data:            ad2,
			},
			want: false, // Different root is different bitlist.
		},
		{
			att: &zondpb.Attestation{
				AggregationBits: bitfield.Bitlist{0b11111001},
				Data:            ad1,
			},
			want: false, // Sanity test that an attestation of different lengths does not panic.
		},
	}

	s, err := NewService(context.Background(), &Config{Pool: NewPool()})
	require.NoError(t, err)

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, err := s.seen(tt.att)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
