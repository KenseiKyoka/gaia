package stake

import (
	"github.com/tendermint/basecoin"
	"github.com/tendermint/basecoin/modules/coin"
)

// Tx
//--------------------------------------------------------------------------------

// register the tx type with it's validation logic
// make sure to use the name of the handler as the prefix in the tx type,
// so it gets routed properly
const (
	ByteTxBond     = 0x55
	ByteTxUnbond   = 0x56
	ByteTxNominate = 0x57
	ByteTxModComm  = 0x58
	TypeTxBond     = Name + "/bond"
	TypeTxUnbond   = Name + "/unbond"
	TypeTxNominate = Name + "/nominate"
	TypeTxModComm  = Name + "/modComm" //modify commission rate
)

func init() {
	basecoin.TxMapper.RegisterImplementation(TxBond{}, TypeTxBond, ByteTxBond)
	basecoin.TxMapper.RegisterImplementation(TxUnbond{}, TypeTxUnbond, ByteTxUnbond)
	basecoin.TxMapper.RegisterImplementation(TxNominate{}, TypeTxNominate, ByteTxNominate)
	basecoin.TxMapper.RegisterImplementation(TxModComm{}, TypeTxModComm, ByteTxModComm)
}

//Verify interface at compilation
var _, _, _, _ basecoin.TxInner = &TxBond{}, &TxUnbond{}, &TxNominate{}, &TxModComm{}

/////////////////////////////////////////////////////////////////
// TxBond

// TxBond - struct for all staking transactions
type TxBond struct {
	Validator basecoin.Actor `json:"validator"`
	Amount    coin.Coin      `json:"amount"`
}

// NewTxBond - return a new counter transaction struct wrapped as a basecoin transaction
func NewTxBond(validator basecoin.Actor, amount coin.Coins) basecoin.Tx {
	return TxBond{
		Validator: validator,
		Amount:    amount,
	}.Wrap()
}

// Wrap - Wrap a Tx as a Basecoin Tx
func (tx TxBond) Wrap() basecoin.Tx {
	return basecoin.Tx{c}
}

// ValidateBasic - Check the bonding coins, Validator is non-empty
func (tx TxBond) ValidateBasic() error {
	if tx.Validator.Empty() {
		return errValidatorEmpty
	}
	if !tx.Amount.IsValid() {
		return coin.ErrInvalidCoins()
	}
	if !tx.Amount.IsNonnegative() {
		return coin.ErrInvalidCoins()
	}
	return nil
}

/////////////////////////////////////////////////////////////////
// TxUnbond

// TxUnbond - struct for all staking transactions
type TxUnbond struct {
	Validator basecoin.Actor `json:"validator"`
	Amount    coin.Coin      `json:"amount"`
}

// NewTxUnbond - return a new counter transaction struct wrapped as a basecoin transaction
func NewTxUnbond(validator basecoin.Actor, amount coin.Coins) basecoin.Tx {
	return TxUnbond{
		Validator: validator,
		Amount:    amount,
	}.Wrap()
}

// Wrap - Wrap a Tx as a Basecoin Tx
func (tx TxUnbond) Wrap() basecoin.Tx {
	return basecoin.Tx{tx}
}

// ValidateBasic - Check coins as well as that you have coins in the validator
func (tx TxUnbond) ValidateBasic() error {
	if tx.Validator.Empty() {
		return errValidatorEmpty
	}
	if !tx.Amount.IsValid() {
		return coin.ErrInvalidCoins()
	}
	if !tx.Amount.IsNonnegative() {
		return coin.ErrInvalidCoins()
	}
	return nil
}

/////////////////////////////////////////////////////////////////
// TxNominate

// TxNominate - struct for all staking transactions
type TxNominate struct {
	Validator  basecoin.Actor `json:"validator"`
	Amount     coin.Coin      `json:"amount"`
	Commission uint64         `json:"commission"`
}

// NewTxNominate - return a new counter transaction struct wrapped as a basecoin transaction
func NewTxNominate(validator basecoin.Actor, amount coin.Coins, commission int) basecoin.Tx {
	return TxNominate{
		Validator:  validator,
		Amount:     amount,
		Commission: commission,
	}.Wrap()
}

// Wrap - Wrap a Tx as a Basecoin Tx
func (tx TxNominate) Wrap() basecoin.Tx {
	return basecoin.Tx{tx}
}

// ValidateBasic - Check coins as well as that the validator is actually a validator
// TODO validate commission is not negative and valid
func (tx TxNominate) ValidateBasic() error {
	if tx.Validator.Empty() {
		return errValidatorEmpty
	}
	if !tx.Amount.IsValid() {
		return coin.ErrInvalidCoins()
	}
	if !tx.Amount.IsNonnegative() {
		return coin.ErrInvalidCoins()
	}
	return nil
}

/////////////////////////////////////////////////////////////////
// TxModComm

// TxModComm - struct for all staking transactions
type TxModComm struct {
	Validator  basecoin.Actor `json:"validator"`
	Commission uint64         `json:"commission"`
}

// NewTxModComm - return a new counter transaction struct wrapped as a basecoin transaction
func NewTxModComm(validator basecoin.Actor, commission string) basecoin.Tx {
	return TxModComm{
		Validator:  validator,
		Commission: commission,
	}.Wrap()
}

// Wrap - Wrap a Tx as a Basecoin Tx
func (tx TxModComm) Wrap() basecoin.Tx {
	return basecoin.Tx{tx}
}

// ValidateBasic - Check coins as well as that the validator is actually a validator
// TODO validate commission is not negative and valid
func (tx TxModComm) ValidateBasic() error {
	if tx.Validator.Empty() {
		return errValidatorEmpty
	}
	return nil
}
