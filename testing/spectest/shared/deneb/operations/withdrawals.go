package operations

import (
	"context"
	"path"
	"testing"

	"github.com/golang/snappy"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/blocks"
	"github.com/theQRL/qrysm/v4/beacon-chain/state"
	consensusblocks "github.com/theQRL/qrysm/v4/consensus-types/blocks"
	"github.com/theQRL/qrysm/v4/consensus-types/interfaces"
	enginev1 "github.com/theQRL/qrysm/v4/proto/engine/v1"
	zondpb "github.com/theQRL/qrysm/v4/proto/prysm/v1alpha1"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/spectest/utils"
	"github.com/theQRL/qrysm/v4/testing/util"
)

func RunWithdrawalsTest(t *testing.T, config string) {
	require.NoError(t, utils.SetConfig(t, config))
	testFolders, testsFolderPath := utils.TestFolders(t, config, "deneb", "operations/withdrawals/pyspec_tests")
	for _, folder := range testFolders {
		t.Run(folder.Name(), func(t *testing.T) {
			folderPath := path.Join(testsFolderPath, folder.Name())
			payloadFile, err := util.BazelFileBytes(folderPath, "execution_payload.ssz_snappy")
			require.NoError(t, err)
			payloadSSZ, err := snappy.Decode(nil /* dst */, payloadFile)
			require.NoError(t, err, "Failed to decompress")
			payload := &enginev1.ExecutionPayloadDeneb{}
			require.NoError(t, payload.UnmarshalSSZ(payloadSSZ), "Failed to unmarshal")

			body := &zondpb.BeaconBlockBodyDeneb{ExecutionPayload: payload}
			RunBlockOperationTest(t, folderPath, body, func(_ context.Context, s state.BeaconState, b interfaces.SignedBeaconBlock) (state.BeaconState, error) {
				payload, err := b.Block().Body().Execution()
				if err != nil {
					return nil, err
				}
				withdrawals, err := payload.Withdrawals()
				if err != nil {
					return nil, err
				}
				p, err := consensusblocks.WrappedExecutionPayloadDeneb(&enginev1.ExecutionPayloadDeneb{Withdrawals: withdrawals}, 0)
				require.NoError(t, err)
				return blocks.ProcessWithdrawals(s, p)
			})
		})
	}
}
