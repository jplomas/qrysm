package epoch_processing

import (
	"testing"

	"github.com/theQRL/qrysm/v4/testing/spectest/shared/bellatrix/epoch_processing"
)

func TestMainnet_Bellatrix_EpochProcessing_RandaoMixesReset(t *testing.T) {
	epoch_processing.RunRandaoMixesResetTests(t, "mainnet")
}
