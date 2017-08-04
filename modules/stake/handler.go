package stake

import (
	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/basecoin"
	"github.com/tendermint/basecoin/errors"
	"github.com/tendermint/basecoin/modules/auth"
	"github.com/tendermint/basecoin/modules/base"
	"github.com/tendermint/basecoin/modules/coin"
	"github.com/tendermint/basecoin/modules/fee"
	"github.com/tendermint/basecoin/modules/ibc"
	"github.com/tendermint/basecoin/modules/nonce"
	"github.com/tendermint/basecoin/modules/roles"
	"github.com/tendermint/basecoin/stack"
	"github.com/tendermint/basecoin/state"
	"github.com/tendermint/basecoin/types"
	"github.com/tendermint/go-wire"
)

//nolint
const (
	Name      = "stake"
	Precision = 10e8
)

// NewHandler returns a new counter transaction processing handler
func NewHandler(feeDenom string) basecoin.Handler {
	return stack.New(
		base.Logger{},
		stack.Recovery{},
		auth.Signatures{},
		base.Chain{},
		stack.Checkpoint{OnCheck: true},
		nonce.ReplayCheck{},
	).
		IBC(ibc.NewMiddleware()).
		Apps(
			roles.NewMiddleware(),
			fee.NewSimpleFeeMiddleware(coin.Coin{feeDenom, 0}, fee.Bank),
			stack.Checkpoint{OnDeliver: true},
		).
		Dispatch(
			coin.NewHandler(),
			stack.WrapHandler(roles.NewHandler()),
			stack.WrapHandler(ibc.NewHandler()),
		)
}

// Handler the transaction processing handler
type Handler struct {
	stack.NopOption
}

var _ stack.Dispatchable = Handler{} //enforce interface at compile time

// Name - return stake namespace
func (Handler) Name() string {
	return Name
}

// AssertDispatcher - placeholder for stack.Dispatchable
func (Handler) AssertDispatcher() {}

// CheckTx checks if the tx is properly structured
func (h Handler) CheckTx(ctx basecoin.Context, store state.SimpleDB,
	tx basecoin.Tx, _ basecoin.Checker) (res basecoin.CheckResult, err error) {
	_, err = checkTx(ctx, tx)
	return
}
func checkTx(ctx basecoin.Context, tx basecoin.Tx) (ctr basecoin.Tx, err error) {
	ctr, ok := tx.Unwrap().(Tx)
	if !ok {
		return ctr, errors.ErrInvalidFormat(TypeTx, tx)
	}
	err = ctr.ValidateBasic()
	if err != nil {
		return ctr, err
	}
	return ctr, nil
}

// DeliverTx executes the tx if valid
func (h Handler) DeliverTx(ctx basecoin.Context, store state.SimpleDB,
	tx basecoin.Tx, dispatch basecoin.Deliver) (res basecoin.DeliverResult, err error) {
	ctr, err := checkTx(ctx, tx)
	if err != nil {
		return res, err
	}

	//start by processing the unbonding queue
	height := ctx.BlockHeight()
	processUnbondingQueue(store, height)

	//now actually run the transaction
	var tx Tx
	err := wire.ReadBinaryBytes(txBytes, &tx)
	if err != nil {
		return abci.ErrBaseEncodingError.AppendLog("Error decoding tx: " + err.Error())
	}

	var abciRes abci.Result
	switch txType := tx.(type) {
	case TxBond:
		abciRes, err = sp.runTxBond(txType, store, ctx)
	case TxUnbond:
		abciRes, err = sp.runTxUnbond(txType, store, ctx, height)
	case TxNominate:
		abciRes, err = sp.runTxNominate(txType, store, ctx)
	case TxModComm:
		abciRes, err = sp.runTxModComm(txType, store, ctx)
	}

	//determine the validator set changes
	bondValues := getBondValues(store)
	res = basecoin.DeliverResult{
		Data:    abciRes.Data,
		Log:     abciRes.Log,
		Diff:    bondValues.Validators(), //FIXME this is the full set, need to just use the diff
		GasUsed: 0,                       //TODO add gas accounting
	}

	return res, err
}

///////////////////////////////////////////////////////////////////////////////////////////////////

// Plugin is a proof-of-stake plugin for Basecoin
type Plugin struct {
	UnbondingPeriod uint64 // how long unbonding takes (measured in blocks)
	CoinDenom       string // bondable coin denomination
}

func (sp Plugin) runTxBond(tx TxBond, store state.SimpleDB, ctx types.CallContext) (res abci.Result) {
	if len(ctx.Coins) != 1 {
		return abci.ErrInternalError.AppendLog("Invalid coins")
	}
	if ctx.Coins[0].Denom != sp.CoinDenom {
		return abci.ErrInternalError.AppendLog("Invalid coin denomination")
	}

	// get amount of coins to bond
	coinAmount := ctx.Coins[0].Amount
	if coinAmount <= 0 {
		return abci.ErrInternalError.AppendLog("Amount must be > 0")
	}

	bondAccount := loadBondAccount(store, ctx.CallerAddress, tx.ValidatorPubKey)
	if bondAccount == nil {
		if tx.Sequence != 0 {
			return abci.ErrInternalError.AppendLog("Invalid sequence number")
		}
		// create new account for this (delegator, validator) pair
		bondAccount = &BondAccount{
			Amount:   0,
			Sequence: 0,
		}
	} else if tx.Sequence != (bondAccount.Sequence + 1) {
		return abci.ErrInternalError.AppendLog("Invalid sequence number")
	}

	// add tokens to validator's bond supply
	bondValues := loadBondValues(store)
	_, bondValue := bondValues.Get(tx.ValidatorPubKey)
	if bondValue == nil {
		// first bond for this validator, initialize a new BondValue
		bondValue = &BondValue{
			ValidatorPubKey: tx.ValidatorPubKey,
			Total:           0,
			ExchangeRate:    1 * Precision, // starts at one atom per bond token
		}
		bondValues = append(bondValues, *bondValue)
	}
	// calulcate amount of bond tokens to create, based on exchange rate
	bondAmount := uint64(coinAmount) * Precision / bondValue.ExchangeRate
	bondValue.Total += bondAmount
	bondAccount.Amount += bondAmount
	bondAccount.Sequence++

	// TODO: special rules for entering validator set

	storeBondValues(store, bondValues)
	storeBondAccount(store, ctx.CallerAddress, tx.ValidatorPubKey, bondAccount)

	return abci.OK
}

func (sp Plugin) runTxUnbond(tx TxUnbond, store state.SimpleDB,
	ctx types.CallContext, height uint64) (res abci.Result) {
	if tx.BondAmount <= 0 {
		return abci.ErrInternalError.AppendLog("Unbond amount must be > 0")
	}

	bondAccount := loadBondAccount(store, ctx.CallerAddress, tx.ValidatorPubKey)
	if bondAccount == nil {
		return abci.ErrBaseUnknownAddress.AppendLog("No bond account for this (address, validator) pair")
	}
	if bondAccount.Amount < tx.BondAmount {
		return abci.ErrBaseInsufficientFunds.AppendLog("Insufficient bond tokens")
	}

	// subtract tokens from bond account
	bondAccount.Amount -= tx.BondAmount
	if bondAccount.Amount == 0 {
		removeBondAccount(store, ctx.CallerAddress, tx.ValidatorPubKey)
	} else {
		storeBondAccount(store, ctx.CallerAddress, tx.ValidatorPubKey, bondAccount)
	}

	// subtract tokens from bond value
	bondValues := loadBondValues(store)
	bvIndex, bondValue := bondValues.Get(tx.ValidatorPubKey)
	bondValue.Total -= tx.BondAmount
	if bondValue.Total == 0 {
		bondValues.Remove(bvIndex)
	}
	// will get sorted in EndBlock
	storeBondValues(store, bondValues)

	// add unbond record to queue
	unbond := Unbond{
		ValidatorPubKey: tx.ValidatorPubKey,
		Address:         ctx.CallerAddress,
		BondAmount:      tx.BondAmount,
		HeightAtInit:    height, // will unbond at `height + UnbondingPeriod`
	}
	unbondQueue := loadQueue(store)
	unbondBytes := wire.BinaryBytes(unbond)
	unbondQueue.Push(unbondBytes)

	return abci.OK
}

func (sp Plugin) runNominate(tx TxNominate, store state.SimpleDB, ctx types.CallContext) (res abci.Result) {

	// create bond value object
	bondValue := BondValue{
		ValidatorPubKey: tx.PubKey,
		Commission:      tx.Commission,
		Total:           tx.Amount.Amount,
		ExchangeRate:    1 * Precision,
	}

	//append and store
	bondValues := getDelegatorBonds(store)
	bondValues = append(bondValues, bondValue)
	setDelegatorBonds(store, bondValues)

	return abci.OK
}

//TODO Update logic
func (sp Plugin) runModComm(tx TxModComm, store state.SimpleDB, ctx types.CallContext) (res abci.Result) {

	// create bond value object
	bond := DelegatorBond{
		ValidatorPubKey: tx.PubKey,
		Commission:      tx.Commission,
		Total:           tx.Amount.Amount,
		ExchangeRate:    1 * Precision,
	}

	//append and store
	bonds := getDelegatorBonds(store)
	bonds = append(bonds, bond)
	setDelegatorBonds(store, bondValues)

	return abci.OK
}

// process unbonds which have finished
func (sp Plugin) processUnbondingQueue(store state.SimpleDB, height uint64, err error) {
	queue := loadUnbondQueue(store)

	//Get the peek unbond record from the queue
	unbondBytes := queue.Peek()
	var unbond Unbond
	err = wire.ReadBinaryBytes(unbondBytes, unbond)
	if err != nil {
		return
	}

	for unbond != nil && height-unbond.HeightAtInit > sp.UnbondingPeriod {
		queue.Pop()

		// add unbonded coins to basecoin account, based on current exchange rate
		_, bondValue := loadBondValues(store).Get(unbond.ValidatorPubKey)
		coinAmount := unbond.BondAmount * bondValue.ExchangeRate / Precision
		account := bcs.GetAccount(store, unbond.Address)
		payout := makeCoin(coinAmount, sp.CoinDenom)
		account.Balance = account.Balance.Plus(payout)
		bcs.SetAccount(store, unbond.Address, account)

		// TODO make function variable with the previous time this code is called
		// get next unbond record
		unbondBytes := queue.Peek()
		var unbond Unbond
		err = wire.ReadBinaryBytes(unbondBytes, unbond)
		if err != nil {
			return
		}
	}
}
