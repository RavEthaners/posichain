package services

import (
	"context"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/pkg/errors"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/rosetta/common"
)

// ConstructionParse implements the /construction/parse endpoint.
func (s *ConstructAPI) ConstructionParse(
	ctx context.Context, request *types.ConstructionParseRequest,
) (*types.ConstructionParseResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromString(request.Transaction)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if request.Signed {
		return parseSignedTransaction(ctx, wrappedTransaction, tx)
	}
	return parseUnsignedTransaction(ctx, wrappedTransaction, tx)
}

// parseUnsignedTransaction ..
func parseUnsignedTransaction(
	ctx context.Context, wrappedTransaction *WrappedTransaction, tx hmyTypes.PoolTransaction,
) (*types.ConstructionParseResponse, *types.Error) {
	if wrappedTransaction == nil || tx == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil wrapped transaction or unwrapped transaction",
		})
	}

	// TODO (dm): implement intended receipt for staking transactions
	intendedReceipt := &hmyTypes.Receipt{
		GasUsed: tx.Gas(),
	}
	formattedTx, rosettaError := FormatTransaction(tx, intendedReceipt, wrappedTransaction.ContractCode)
	if rosettaError != nil {
		return nil, rosettaError
	}
	tempAccID, rosettaError := newAccountIdentifier(FormatDefaultSenderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	foundSender := false
	operations := formattedTx.Operations
	for _, op := range operations {
		if types.Hash(op.Account) == types.Hash(tempAccID) {
			foundSender = true
			op.Account = wrappedTransaction.From
		}
		op.Status = ""
	}
	if !foundSender {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "temp sender not found in transaction operations",
		})
	}
	return &types.ConstructionParseResponse{
		Operations: operations,
	}, nil
}

// parseSignedTransaction ..
func parseSignedTransaction(
	ctx context.Context, wrappedTransaction *WrappedTransaction, tx hmyTypes.PoolTransaction,
) (*types.ConstructionParseResponse, *types.Error) {
	if wrappedTransaction == nil || tx == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "nil wrapped transaction or unwrapped transaction",
		})
	}

	// TODO (dm): implement intended receipt for staking transactions
	intendedReceipt := &hmyTypes.Receipt{
		GasUsed: tx.Gas(),
	}
	formattedTx, rosettaError := FormatTransaction(tx, intendedReceipt, wrappedTransaction.ContractCode)
	if rosettaError != nil {
		return nil, rosettaError
	}
	sender, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": errors.WithMessage(err, "unable to get sender address, invalid signed transaction"),
		})
	}
	senderID, rosettaError := newAccountIdentifier(sender)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if types.Hash(senderID) != types.Hash(wrappedTransaction.From) {
		return nil, common.NewError(common.InvalidTransactionConstructionError, map[string]interface{}{
			"message": "wrapped transaction sender/from does not match transaction signer",
		})
	}
	for _, op := range formattedTx.Operations {
		op.Status = ""
	}
	return &types.ConstructionParseResponse{
		Operations:               formattedTx.Operations,
		AccountIdentifierSigners: []*types.AccountIdentifier{senderID},
	}, nil
}
