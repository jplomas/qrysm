package testing

import (
	"context"

	zond "github.com/theQRL/qrysm/v4/proto/prysm/v1alpha1"
)

// MockProtector mocks the protector.
type MockProtector struct {
	AllowAttestation        bool
	AllowBlock              bool
	VerifyAttestationCalled bool
	VerifyBlockCalled       bool
	StatusCalled            bool
}

// CheckAttestationSafety returns bool with allow attestation value.
func (mp MockProtector) CheckAttestationSafety(_ context.Context, _ *zond.IndexedAttestation) bool {
	mp.VerifyAttestationCalled = true // skipcq: RVV-B0006
	return mp.AllowAttestation
}

// CheckBlockSafety returns bool with allow block value.
func (mp MockProtector) CheckBlockSafety(_ context.Context, _ *zond.SignedBeaconBlockHeader) bool {
	mp.VerifyBlockCalled = true // skipcq: RVV-B0006
	return mp.AllowBlock
}

// Status returns nil.
func (mp MockProtector) Status() error {
	mp.StatusCalled = true // skipcq: RVV-B0006
	return nil
}
