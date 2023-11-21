package stakerservice

type ResultHealth struct{}

type ResultStake struct {
	TxHash string `json:"tx_hash"`
}

type StakingDetails struct {
	StakingTxHash  string `json:"staking_tx_hash"`
	StakerAddress  string `json:"staker_address"`
	ChangeAddress  string `json:"change_address"`
	StakingScript  string `json:"staking_script"`
	StakingState   string `json:"staking_state"`
	Watched        bool   `json:"watched"`
	TransactionIdx string `json:"transaction_idx"`
}

type OutputDetail struct {
	Amount  string `json:"amount"`
	Address string `json:"address"`
}

type OutputsResponse struct {
	Outputs []OutputDetail `json:"outputs"`
}
type SpendTxDetails struct {
	TxHash  string `json:"tx_hash"`
	TxValue string `json:"tx_value"`
}

type ValidatorInfoResponse struct {
	// Hex encoded Babylon public secp256k1 key in compressed format
	BabylonPublicKey string `json:"babylon_public_Key"`
	// Hex encoded Bitcoin public secp256k1 key in BIP340 format
	BtcPublicKey string `json:"bitcoin_public_Key"`
}

type ValidatorsResponse struct {
	Validators          []ValidatorInfoResponse `json:"validators"`
	TotalValidatorCount string                  `json:"total_validator_count"`
}

type ListStakingTransactionsResponse struct {
	Transactions          []StakingDetails `json:"transactions"`
	TotalTransactionCount string           `json:"total_transaction_count"`
}

type UnbondingResponse struct {
	UnbondingTxHash string `json:"unbonding_tx_hash"`
}

type WithdrawableTransactionsResponse struct {
	Transactions                     []StakingDetails `json:"transactions"`
	LastWithdrawableTransactionIndex string           `json:"last_transaction_index"`
	TotalTransactionCount            string           `json:"total_transaction_count"`
}
