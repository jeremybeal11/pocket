package test_artifacts

// Cross module imports are okay because this is only used for testing and not business logic
import (
	"fmt"
	"strconv"

	typesCons "github.com/pokt-network/pocket/consensus/types"
	typesP2P "github.com/pokt-network/pocket/p2p/types"
	typesPers "github.com/pokt-network/pocket/persistence/types"
	"github.com/pokt-network/pocket/runtime"
	"github.com/pokt-network/pocket/runtime/defaults"
	"github.com/pokt-network/pocket/shared/crypto"
	"github.com/pokt-network/pocket/shared/modules"
	typesTelemetry "github.com/pokt-network/pocket/telemetry"
	"github.com/pokt-network/pocket/utility/types"
	typesUtil "github.com/pokt-network/pocket/utility/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// INVESTIGATE: It seems improperly scoped that the modules have to have shared 'testing' code
//  It might be an inevitability to have shared testing code, but would like more eyes on it.
//  Look for opportunities to make testing completely modular

// TODO (Team) this is meant to be a **temporary** replacement for the recently deprecated
// 'genesis config' option. We need to implement a real suite soon!
func NewGenesisState(numValidators, numServiceNodes, numApplications, numFisherman int) (modules.GenesisState, []string) {
	apps, appsPrivateKeys := NewActors(types.ActorType_App, numApplications)
	vals, validatorPrivateKeys := NewActors(types.ActorType_Validator, numValidators)
	serviceNodes, snPrivateKeys := NewActors(types.ActorType_ServiceNode, numServiceNodes)
	fish, fishPrivateKeys := NewActors(types.ActorType_Fisherman, numFisherman)

	genesisState := runtime.NewGenesis(
		&typesCons.ConsensusGenesisState{
			GenesisTime:   timestamppb.Now(),
			ChainId:       defaults.DefaultChainID,
			MaxBlockBytes: defaults.DefaultMaxBlockBytes,
			Validators:    typesCons.ToConsensusValidators(vals),
		},
		&typesPers.PersistenceGenesisState{
			Pools:        typesPers.ToPersistenceAccounts(NewPools()),
			Accounts:     typesPers.ToPersistenceAccounts(NewAccounts(numValidators+numServiceNodes+numApplications+numFisherman, append(append(append(validatorPrivateKeys, snPrivateKeys...), fishPrivateKeys...), appsPrivateKeys...)...)), // TODO(olshansky): clean this up
			Applications: typesPers.ToPersistenceActors(apps),
			Validators:   typesPers.ToPersistenceActors(vals),
			ServiceNodes: typesPers.ToPersistenceActors(serviceNodes),
			Fishermen:    typesPers.ToPersistenceActors(fish),
			Params:       typesPers.ToPersistenceParams(DefaultParams()),
		},
	)

	return genesisState, validatorPrivateKeys
}

func NewDefaultConfigs(privateKeys []string) (configs []modules.Config) {
	for i, pk := range privateKeys {
		configs = append(configs, NewDefaultConfig(i, pk))
	}
	return
}

func NewDefaultConfig(i int, pk string) modules.Config {
	return runtime.NewConfig(
		&runtime.BaseConfig{
			RootDirectory: "/go/src/github.com/pocket-network",
			PrivateKey:    pk,
		},
		runtime.WithConsensusConfig(
			&typesCons.ConsensusConfig{
				MaxMempoolBytes: 500000000,
				PacemakerConfig: &typesCons.PacemakerConfig{
					TimeoutMsec:               5000,
					Manual:                    true,
					DebugTimeBetweenStepsMsec: 1000,
				},
				PrivateKey: pk,
			}),
		runtime.WithUtilityConfig(&typesUtil.UtilityConfig{
			MaxMempoolTransactionBytes: 1024 * 1024 * 1024, // 1GB V0 defaults
			MaxMempoolTransactions:     9000,
		}),
		runtime.WithPersistenceConfig(&typesPers.PersistenceConfig{
			PostgresUrl:    "postgres://postgres:postgres@pocket-db:5432/postgres",
			NodeSchema:     "node" + strconv.Itoa(i+1),
			BlockStorePath: "/var/blockstore",
		}),
		runtime.WithP2PConfig(&typesP2P.P2PConfig{
			ConsensusPort:         8080,
			UseRainTree:           true,
			IsEmptyConnectionType: false,
			PrivateKey:            pk,
		}),
		runtime.WithTelemetryConfig(&typesTelemetry.TelemetryConfig{
			Enabled:  true,
			Address:  "0.0.0.0:9000",
			Endpoint: "/metrics",
		}),
	)
}

func NewPools() (pools []modules.Account) { // TODO (Team) in the real testing suite, we need to populate the pool amounts dependent on the actors
	for _, name := range typesPers.PoolNames_name {
		if name == typesPers.PoolNames_FeeCollector.String() {
			pools = append(pools, &typesPers.Account{
				Address: name,
				Amount:  "0",
			})
			continue
		}
		pools = append(pools, &typesPers.Account{
			Address: name,
			Amount:  defaults.DefaultAccountAmountString,
		})
	}
	return
}

func NewAccounts(n int, privateKeys ...string) (accounts []modules.Account) {
	for i := 0; i < n; i++ {
		_, _, addr := GenerateNewKeysStrings()
		if privateKeys != nil {
			pk, _ := crypto.NewPrivateKey(privateKeys[i])
			addr = pk.Address().String()
		}
		accounts = append(accounts, &typesPers.Account{
			Address: addr,
			Amount:  defaults.DefaultAccountAmountString,
		})
	}
	return
}

// TODO: The current implementation of NewActors  will have overlapping `ServiceUrl` for different
//       types of actors which needs to be fixed.
func NewActors(actorType typesUtil.ActorType, n int) (actors []modules.Actor, privateKeys []string) {
	for i := 0; i < n; i++ {
		genericParam := getServiceUrl(i + 1)
		if int32(actorType) == int32(types.ActorType_App) {
			genericParam = defaults.DefaultMaxRelaysString
		}
		actor, pk := NewDefaultActor(int32(actorType), genericParam)
		actors = append(actors, actor)
		privateKeys = append(privateKeys, pk)
	}

	return
}

func getServiceUrl(n int) string {
	return fmt.Sprintf(defaults.ServiceUrlFormat, n)
}

func NewDefaultActor(actorType int32, genericParam string) (actor modules.Actor, privateKey string) {
	privKey, pubKey, addr := GenerateNewKeysStrings()
	chains := defaults.DefaultChains
	if actorType == int32(typesPers.ActorType_Val) {
		chains = nil
	} else if actorType == int32(types.ActorType_App) {
		genericParam = defaults.DefaultMaxRelaysString
	}
	return &typesPers.Actor{
		Address:         addr,
		PublicKey:       pubKey,
		Chains:          chains,
		GenericParam:    genericParam,
		StakedAmount:    defaults.DefaultStakeAmountString,
		PausedHeight:    defaults.DefaultPauseHeight,
		UnstakingHeight: defaults.DefaultUnstakingHeight,
		Output:          addr,
		ActorType:       typesPers.ActorType(actorType),
	}, privKey
}

func GenerateNewKeys() (privateKey crypto.PrivateKey, publicKey crypto.PublicKey, address crypto.Address) {
	privateKey, _ = crypto.GeneratePrivateKey()
	publicKey = privateKey.PublicKey()
	address = publicKey.Address()
	return
}

func GenerateNewKeysStrings() (privateKey, publicKey, address string) {
	privKey, pubKey, addr := GenerateNewKeys()
	privateKey = privKey.String()
	publicKey = pubKey.String()
	address = addr.String()
	return
}
