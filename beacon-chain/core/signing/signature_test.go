package signing_test

import (
	"testing"
	"time"

	"github.com/theQRL/qrysm/v4/beacon-chain/core/signing"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/crypto/bls"
	"github.com/theQRL/qrysm/v4/encoding/bytesutil"
	zondpb "github.com/theQRL/qrysm/v4/proto/prysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/require"
)

func TestVerifyRegistrationSignature(t *testing.T) {
	sk, err := bls.RandKey()
	require.NoError(t, err)
	reg := &zondpb.ValidatorRegistrationV1{
		FeeRecipient: bytesutil.PadTo([]byte("fee"), 20),
		GasLimit:     123456,
		Timestamp:    uint64(time.Now().Unix()),
		Pubkey:       sk.PublicKey().Marshal(),
	}
	d := params.BeaconConfig().DomainApplicationBuilder
	domain, err := signing.ComputeDomain(d, nil, nil)
	require.NoError(t, err)
	sr, err := signing.ComputeSigningRoot(reg, domain)
	require.NoError(t, err)
	sk.Sign(sr[:]).Marshal()

	sReg := &zondpb.SignedValidatorRegistrationV1{
		Message:   reg,
		Signature: sk.Sign(sr[:]).Marshal(),
	}
	require.NoError(t, signing.VerifyRegistrationSignature(sReg))

	sReg.Signature = []byte("bad")
	require.ErrorIs(t, signing.VerifyRegistrationSignature(sReg), signing.ErrSigFailedToVerify)

	sReg.Message = nil
	require.ErrorIs(t, signing.VerifyRegistrationSignature(sReg), signing.ErrNilRegistration)
}
