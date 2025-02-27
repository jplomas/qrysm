package fork

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/snappy"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/helpers"
	"github.com/theQRL/qrysm/v4/beacon-chain/core/transition"
	state_native "github.com/theQRL/qrysm/v4/beacon-chain/state/state-native"
	"github.com/theQRL/qrysm/v4/config/params"
	"github.com/theQRL/qrysm/v4/consensus-types/blocks"
	types "github.com/theQRL/qrysm/v4/consensus-types/primitives"
	zondpb "github.com/theQRL/qrysm/v4/proto/zond/v2"
	"github.com/theQRL/qrysm/v4/testing/require"
	"github.com/theQRL/qrysm/v4/testing/spectest/utils"
	"github.com/theQRL/qrysm/v4/testing/util"
)

type ForkConfig struct {
	PostFork    string `json:"post_fork"`
	ForkEpoch   int    `json:"fork_epoch"`
	ForkBlock   *int   `json:"fork_block"`
	BlocksCount int    `json:"blocks_count"`
}

// RunForkTransitionTest is a helper function that runs deneb's transition core tests.
func RunForkTransitionTest(t *testing.T, config string) {
	params.SetupTestConfigCleanup(t)
	require.NoError(t, utils.SetConfig(t, config))

	testFolders, testsFolderPath := utils.TestFolders(t, config, "deneb", "transition/core/pyspec_tests")
	for _, folder := range testFolders {
		t.Run(folder.Name(), func(t *testing.T) {
			helpers.ClearCache()
			file, err := util.BazelFileBytes(testsFolderPath, folder.Name(), "meta.yaml")
			require.NoError(t, err)
			config := &ForkConfig{}
			require.NoError(t, utils.UnmarshalYaml(file, config), "Failed to Unmarshal")

			preforkBlocks := make([]*zondpb.SignedBeaconBlockCapella, 0)
			postforkBlocks := make([]*zondpb.SignedBeaconBlockDeneb, 0)
			// Fork happens without any pre-fork blocks.
			if config.ForkBlock == nil {
				for i := 0; i < config.BlocksCount; i++ {
					fileName := fmt.Sprint("blocks_", i, ".ssz_snappy")
					blockFile, err := util.BazelFileBytes(testsFolderPath, folder.Name(), fileName)
					require.NoError(t, err)
					blockSSZ, err := snappy.Decode(nil /* dst */, blockFile)
					require.NoError(t, err, "Failed to decompress")
					block := &zondpb.SignedBeaconBlockDeneb{}
					require.NoError(t, block.UnmarshalSSZ(blockSSZ), "Failed to unmarshal")
					postforkBlocks = append(postforkBlocks, block)
				}
				// Fork happens with pre-fork blocks.
			} else {
				for i := 0; i <= *config.ForkBlock; i++ {
					fileName := fmt.Sprint("blocks_", i, ".ssz_snappy")
					blockFile, err := util.BazelFileBytes(testsFolderPath, folder.Name(), fileName)
					require.NoError(t, err)
					blockSSZ, err := snappy.Decode(nil /* dst */, blockFile)
					require.NoError(t, err, "Failed to decompress")
					block := &zondpb.SignedBeaconBlockCapella{}
					require.NoError(t, block.UnmarshalSSZ(blockSSZ), "Failed to unmarshal")
					preforkBlocks = append(preforkBlocks, block)
				}
				for i := *config.ForkBlock + 1; i < config.BlocksCount; i++ {
					fileName := fmt.Sprint("blocks_", i, ".ssz_snappy")
					blockFile, err := util.BazelFileBytes(testsFolderPath, folder.Name(), fileName)
					require.NoError(t, err)
					blockSSZ, err := snappy.Decode(nil /* dst */, blockFile)
					require.NoError(t, err, "Failed to decompress")
					block := &zondpb.SignedBeaconBlockDeneb{}
					require.NoError(t, block.UnmarshalSSZ(blockSSZ), "Failed to unmarshal")
					postforkBlocks = append(postforkBlocks, block)
				}
			}

			preBeaconStateFile, err := util.BazelFileBytes(testsFolderPath, folder.Name(), "pre.ssz_snappy")
			require.NoError(t, err)
			preBeaconStateSSZ, err := snappy.Decode(nil /* dst */, preBeaconStateFile)
			require.NoError(t, err, "Failed to decompress")
			beaconStateBase := &zondpb.BeaconStateCapella{}
			require.NoError(t, beaconStateBase.UnmarshalSSZ(preBeaconStateSSZ), "Failed to unmarshal")
			beaconState, err := state_native.InitializeFromProtoCapella(beaconStateBase)
			require.NoError(t, err)

			bc := params.BeaconConfig().Copy()
			bc.DenebForkEpoch = types.Epoch(config.ForkEpoch)
			params.OverrideBeaconConfig(bc)

			ctx := context.Background()
			var ok bool
			for _, b := range preforkBlocks {
				wsb, err := blocks.NewSignedBeaconBlock(b)
				require.NoError(t, err)
				st, err := transition.ExecuteStateTransition(ctx, beaconState, wsb)
				require.NoError(t, err)
				beaconState, ok = st.(*state_native.BeaconState)
				require.Equal(t, true, ok)
			}

			for _, b := range postforkBlocks {
				wsb, err := blocks.NewSignedBeaconBlock(b)
				require.NoError(t, err)
				st, err := transition.ExecuteStateTransition(ctx, beaconState, wsb)
				require.NoError(t, err)
				beaconState, ok = st.(*state_native.BeaconState)
				require.Equal(t, true, ok)
			}

			postBeaconStateFile, err := util.BazelFileBytes(testsFolderPath, folder.Name(), "post.ssz_snappy")
			require.NoError(t, err)
			postBeaconStateSSZ, err := snappy.Decode(nil /* dst */, postBeaconStateFile)
			require.NoError(t, err, "Failed to decompress")
			postBeaconState := &zondpb.BeaconStateDeneb{}
			require.NoError(t, postBeaconState.UnmarshalSSZ(postBeaconStateSSZ), "Failed to unmarshal")

			pbState, err := state_native.ProtobufBeaconStateDeneb(beaconState.ToProto())
			require.NoError(t, err)
			require.DeepEqual(t, postBeaconState, pbState, "Did not get expected post state")
		})
	}
}
