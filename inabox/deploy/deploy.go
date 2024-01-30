package deploy

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strconv"

	"github.com/zero-gravity-labs/zerog-data-avail/core"
)

const (
	churnerImage   = "ghcr.io/zero-gravity-labs/zerog-storage-client/churner:local"
	disImage       = "ghcr.io/zero-gravity-labs/zerog-storage-client/disperser:local"
	encoderImage   = "ghcr.io/zero-gravity-labs/zerog-storage-client/encoder:local"
	batcherImage   = "ghcr.io/zero-gravity-labs/zerog-storage-client/batcher:local"
	nodeImage      = "ghcr.io/zero-gravity-labs/zerog-storage-client/node:local"
	retrieverImage = "ghcr.io/zero-gravity-labs/zerog-storage-client/retriever:local"
)

func (env *Config) getKeyString(name string) string {
	key, _ := env.getKey(name)
	keyInt, ok := new(big.Int).SetString(key, 0)
	if !ok {
		log.Panicf("Error: could not parse key %s", key)
	}
	return keyInt.String()
}

func (env *Config) generateZGDADeployConfig() ZGDADeployConfig {

	operators := make([]string, 0)
	stakers := make([]string, 0)
	maxOperatorCount := env.Services.Counts.NumMaxOperatorCount

	numStrategies := 2
	total := float32(0)
	stakes := make([][]string, numStrategies)

	for _, stake := range env.Services.Stakes.Distribution {
		total += stake
	}

	for i := 0; i < numStrategies; i++ {
		stakes[i] = make([]string, len(env.Services.Stakes.Distribution))
		for ind, stake := range env.Services.Stakes.Distribution {
			stakes[i][ind] = strconv.FormatFloat(float64(stake/total*env.Services.Stakes.Total), 'f', 0, 32)
		}
	}

	for i := 0; i < len(env.Services.Stakes.Distribution); i++ {
		stakerName := fmt.Sprintf("staker%d", i)
		operatorName := fmt.Sprintf("opr%d", i)

		stakers = append(stakers, env.getKeyString(stakerName))
		operators = append(operators, env.getKeyString(operatorName))
	}

	config := ZGDADeployConfig{
		UseDefaults:         true,
		NumStrategies:       numStrategies,
		MaxOperatorCount:    maxOperatorCount,
		StakerPrivateKeys:   stakers,
		StakerTokenAmounts:  stakes,
		OperatorPrivateKeys: operators,
	}

	return config

}

func (env *Config) deployZGDAContracts() {
	log.Print("Deploy the ZGDA and ZeroGLayer contracts")

	// get deployer
	deployer, ok := env.GetDeployer(env.ZGDA.Deployer)
	if !ok {
		log.Panicf("Deployer improperly configured")
	}

	changeDirectory(filepath.Join(env.rootPath, "contracts"))

	zgdaDeployConfig := env.generateZGDADeployConfig()
	data, err := json.Marshal(&zgdaDeployConfig)
	if err != nil {
		log.Panicf("Error: %s", err.Error())
	}
	writeFile("script/zgda_deploy_config.json", data)

	execForgeScript("script/SetUpZGDA.s.sol:SetupZGDA", env.Pks.EcdsaMap[deployer.Name].PrivateKey, deployer, nil)

	//add relevant addresses to path
	data = readFile("script/output/zgda_deploy_output.json")
	err = json.Unmarshal(data, &env.ZGDA)
	if err != nil {
		log.Panicf("Error: %s", err.Error())
	}
	blobHeader := &core.BlobHeader{
		QuorumInfos: []*core.BlobQuorumInfo{
			{
				SecurityParam: core.SecurityParam{
					QuorumID:           0,
					AdversaryThreshold: 80,
					QuorumThreshold:    100,
				},
			},
		},
	}
	hash, err := blobHeader.GetQuorumBlobParamsHash()
	if err != nil {
		log.Panicf("Error: %s", err.Error())
	}
	hashStr := fmt.Sprintf("%x", hash)
	execForgeScript("script/MockRollupDeployer.s.sol:MockRollupDeployer", env.Pks.EcdsaMap[deployer.Name].PrivateKey, deployer, []string{"--sig", "run(address,bytes32,uint256)", env.ZGDA.ServiceManager, hashStr, big.NewInt(1e18).String()})

	//add rollup address to path
	data = readFile("script/output/mock_rollup_deploy_output.json")
	var rollupAddr struct{ MockRollup string }
	err = json.Unmarshal(data, &rollupAddr)
	if err != nil {
		log.Panicf("Error: %s", err.Error())
	}

	env.MockRollup = rollupAddr.MockRollup
}

// Deploys a ZGDA experiment
func (env *Config) DeployExperiment() {
	changeDirectory(filepath.Join(env.rootPath, "inabox"))
	defer env.SaveTestConfig()

	log.Print("Deploying experiment...")

	// Log to file
	f, err := os.OpenFile(filepath.Join(env.Path, "deploy.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Panicf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	// Create a new experiment and deploy the contracts

	err = env.loadPrivateKeys()
	if err != nil {
		log.Panicf("could not load private keys: %v", err)
	}

	if env.ZGDA.Deployer != "" && !env.IsZGDADeployed() {
		fmt.Println("Deploying ZGDA")
		env.deployZGDAContracts()
	}

	if deployer, ok := env.GetDeployer(env.ZGDA.Deployer); ok && deployer.DeploySubgraphs {
		startBlock := GetLatestBlockNumber(env.Deployers[0].RPC)
		env.deploySubgraphs(startBlock)
	}

	fmt.Println("Generating variables")
	env.GenerateAllVariables()

	fmt.Println("Test environment has succesfully deployed!")
}

// TODO: Supply the test path to the runner utility
func (env *Config) StartBinaries() {
	changeDirectory(filepath.Join(env.rootPath, "inabox"))
	err := execCmd("./bin.sh", []string{"start-detached"}, []string{})

	if err != nil {
		log.Panicf("Failed to start binaries. Err: %s", err)
	}
}

// TODO: Supply the test path to the runner utility
func (env *Config) StopBinaries() {
	changeDirectory(filepath.Join(env.rootPath, "inabox"))
	err := execCmd("./bin.sh", []string{"stop"}, []string{})
	if err != nil {
		log.Panicf("Failed to stop binaries. Err: %s", err)
	}
}

func (env *Config) StartAnvil() {
	changeDirectory(filepath.Join(env.rootPath, "inabox"))
	err := execCmd("./bin.sh", []string{"start-anvil"}, []string{})
	if err != nil {
		log.Panicf("Failed to start anvil. Err: %s", err)
	}
}

func (env *Config) StopAnvil() {
	changeDirectory(filepath.Join(env.rootPath, "inabox"))
	err := execCmd("./bin.sh", []string{"stop-anvil"}, []string{})
	if err != nil {
		log.Panicf("Failed to stop anvil. Err: %s", err)
	}
}

func (env *Config) RunNodePluginBinary(operation string, operator OperatorVars) {
	changeDirectory(filepath.Join(env.rootPath, "inabox"))

	socket := string(core.MakeOperatorSocket(operator.NODE_HOSTNAME, operator.NODE_DISPERSAL_PORT, operator.NODE_RETRIEVAL_PORT))

	envVars := []string{
		"NODE_OPERATION=" + operation,
		"NODE_ECDSA_KEY_FILE=" + operator.NODE_ECDSA_KEY_FILE,
		"NODE_BLS_KEY_FILE=" + operator.NODE_BLS_KEY_FILE,
		"NODE_ECDSA_KEY_PASSWORD=" + operator.NODE_ECDSA_KEY_PASSWORD,
		"NODE_BLS_KEY_PASSWORD=" + operator.NODE_BLS_KEY_PASSWORD,
		"NODE_SOCKET=" + socket,
		"NODE_QUORUM_ID_LIST=" + operator.NODE_QUORUM_ID_LIST,
		"NODE_CHAIN_RPC=" + operator.NODE_CHAIN_RPC,
		"NODE_BLS_OPERATOR_STATE_RETRIVER=" + operator.NODE_BLS_OPERATOR_STATE_RETRIVER,
		"NODE_ZGDA_SERVICE_MANAGER=" + operator.NODE_ZGDA_SERVICE_MANAGER,
		"NODE_CHURNER_URL=" + operator.NODE_CHURNER_URL,
		"NODE_NUM_CONFIRMATIONS=0",
	}

	err := execCmd("./node-plugin.sh", []string{}, envVars)

	if err != nil {
		log.Panicf("Failed to run node plugin. Err: %s", err)
	}
}
