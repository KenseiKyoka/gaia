package stake

import (
	"bytes"
	"sort"

	abci "github.com/tendermint/abci/types"
)

//TODO make genesis parameter
const maxValidators = 100

// DelegatorBond defines an total amount of bond tokens and their exchange rate to
// coins, associated with a single validator. Interest increases the exchange
// rate, and slashing decreases it. When coins are delegated to this validator,
// the delegator is credited with bond tokens based on amount of coins
// delegated and the current exchange rate. Voting power can be calculated as
// total bonds multiplied by exchange rate.
type DelegatorBond struct {
	ValidatorPubKey []byte
	Commission      string
	Total           uint64 // Number of bond tokens for this validator
	ExchangeRate    uint64 // Exchange rate for this validator's bond tokens (in millionths of coins)
}

// VotingPower - voting power based onthe bond value
func (bc DelegatorBond) VotingPower() uint64 {
	return bc.Total * bc.ExchangeRate / Precision
}

// Validator - Get the validator from a bond value
func (bc DelegatorBond) Validator() *abci.Validator {
	return &abci.Validator{
		PubKey: bc.ValidatorPubKey,
		Power:  bc.VotingPower(),
	}
}

//--------------------------------------------------------------------------------

// DelegatorBonds - the set of all DelegatorBonds
type DelegatorBonds []DelegatorBond

var _ sort.Interface = DelegatorBonds{} //enforce the sort interface at compile time

// nolint - sort interface functions
func (bvs DelegatorBonds) Len() int      { return len(bvs) }
func (bvs DelegatorBonds) Swap(i, j int) { bvs[i], bvs[j] = bvs[j], bvs[i] }
func (bvs DelegatorBonds) Less(i, j int) bool {
	vp1, vp2 := bvs[i].VotingPower(), bvs[j].VotingPower()
	if vp1 == vp2 {
		return bytes.Compare(bvs[i].ValidatorPubKey, bvs[j].ValidatorPubKey) == -1
	}
	return vp1 > vp2
}

// Sort - Sort the array of bonded values
func (bvs DelegatorBonds) Sort() {
	sort.Sort(bvs)
}

// Validators - get the active validator list from the array of DelegatorBonds
func (bvs DelegatorBonds) Validators() []*abci.Validator {
	validators := make([]*abci.Validator, 0, maxValidators)
	for i, bv := range bvs {
		if i == maxValidators {
			break
		}
		validators = append(validators, bv.Validator())
	}
	return validators
}

// Get - get a DelegatorBond for a specific validator from the DelegatorBonds
func (bvs DelegatorBonds) Get(validatorPubKey []byte) (int, *DelegatorBond) {
	for i, bv := range bvs {
		if bytes.Equal(bv.ValidatorPubKey, validatorPubKey) {
			return i, &bv
		}
	}
	return 0, nil
}

// Remove - remove delegator from the delegator list
func (bvs DelegatorBonds) Remove(i int) DelegatorBonds {
	return append(bvs[:i], bvs[i+1:]...)
}

//--------------------------------------------------------------------------------

// DelegateeBond defines an account of bond tokens. It is owned by one delegatee
// account, and is associated with one delegator/validator account
type DelegateeBond struct {
	ValidatorPubKey []byte
	Amount          uint64 // amount of bond tokens
}

// DelegateeBonds - all delegatee bonds
type DelegateeBonds []DelegateeBonds

//Unbond defines an amount of bond tokens which are in the unbonding period
type Unbond struct {
	ValidatorPubKey []byte
	Address         []byte // account to pay out to
	Amount          uint64 // amount of bond tokens which are unbonding
	HeightAtInit    uint64 // when the unbonding started
}
