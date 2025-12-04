package exchange

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/samber/mo"
	"github.com/vmihailenco/msgpack/v5"
)

// signL1Action signs an action using EIP-712 typed data signing
// This implements the L1 action signing mechanism used by Hyperliquid
func signL1Action[T any](
	action T,
	nonce uint64,
	privateKey *ecdsa.PrivateKey,
	vaultAddress mo.Option[common.Address],
	expiresAfter mo.Option[time.Duration],
	isMainnet bool,
) (signature, error) {
	actionHash, err := hashAction(
		action,
		vaultAddress,
		nonce,
		expiresAfter,
	)
	if err != nil {
		return signature{}, fmt.Errorf("failed to create action hash: %w", err)
	}

	phantomAgent := constructPhantomAgent(actionHash, isMainnet)
	typedData := l1Payload(phantomAgent)

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return signature{}, fmt.Errorf(
			"failed generating hash for typed data: %w",
			err,
		)
	}

	return signHash(common.BytesToHash(hash), privateKey)
}

// signL1ActionWithVault signs an L1 action with an optional vault address
// override
func signL1ActionWithVault(
	action map[string]any,
	nonce uint64,
	privateKey *ecdsa.PrivateKey,
	vaultAddress mo.Option[common.Address],
	expiresAfter mo.Option[time.Duration],
	isMainnet bool,
) (signature, error) {
	actionHash, err := hashAction(
		action,
		vaultAddress,
		nonce,
		expiresAfter,
	)
	if err != nil {
		return signature{}, fmt.Errorf("failed to create action hash: %w", err)
	}

	phantomAgent := constructPhantomAgent(actionHash, isMainnet)
	typedData := l1Payload(phantomAgent)

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return signature{}, fmt.Errorf(
			"failed generating hash for typed data: %w",
			err,
		)
	}

	return signHash(common.BytesToHash(hash), privateKey)
}

// signMultiSigAction signs a multi-signature action
// func signMultiSigAction(
// 	action map[string]any,
// 	nonce uint64,
// 	privateKey *ecdsa.PrivateKey,
// 	vaultAddress mo.Option[common.Address],
// 	expiresAfter mo.Option[time.Duration],
// 	isMainnet bool,
// ) (signature, error) {
// 	actionHash, err := hashAction(
// 		action,
// 		vaultAddress,
// 		nonce,
// 		expiresAfter,
// 	)
// 	if err != nil {
// 		return signature{}, fmt.Errorf("failed to create action hash: %w", err)
// 	}

// 	phantomAgent := constructPhantomAgent(actionHash, isMainnet)
// 	typedData := l1Payload(phantomAgent)

// 	hash, _, err := apitypes.TypedDataAndHash(typedData)
// 	if err != nil {
// 		return signature{}, fmt.Errorf(
// 			"failed generating hash for typed data: %w",
// 			err,
// 		)
// 	}

// 	return signHash(common.BytesToHash(hash), privateKey)
// }

// The outer signer MUST be an authorized user on multiSigUser
func signMultisigL1ActionPayload[T any](
	action T,
	nonce uint64,
	privateKey *ecdsa.PrivateKey,
	vaultAddress mo.Option[common.Address],
	expiresAfter mo.Option[time.Duration],
	isMainnet bool,
	multiSigUser common.Address,
	outerSigner common.Address,
) (signature, error) {
	envelope := []any{
		strings.ToLower(multiSigUser.Hex()),
		strings.ToLower(outerSigner.Hex()),
		action,
	}

	return signL1Action(
		envelope,
		nonce,
		privateKey,
		vaultAddress,
		expiresAfter,
		isMainnet,
	)
}

func signMultiSigAction(
	action multiSigAction,
	nonce uint64,
	privateKey *ecdsa.PrivateKey,
	vaultAddress mo.Option[common.Address],
	expiresAfter mo.Option[time.Duration],
	isMainnet bool,
) (signature, error) {
	// Create action without type for hashing
	actionWithoutType := struct {
		SignatureChainId string          `json:"signatureChainId"`
		Signatures       []signature     `json:"signatures"`
		Payload          multiSigPayload `json:"payload"`
	}{
		SignatureChainId: action.SignatureChainId,
		Signatures:       action.Signatures,
		Payload:          action.Payload,
	}

	// Hash the action
	actionHash, err := hashAction(
		actionWithoutType,
		vaultAddress,
		nonce,
		expiresAfter,
	)
	if err != nil {
		return signature{}, fmt.Errorf("failed to create action hash: %w", err)
	}

	// Create envelope for signing
	chainName := "Mainnet"
	if !isMainnet {
		chainName = "Testnet"
	}

	envelope := map[string]any{
		"hyperliquidChain":   chainName,
		"multiSigActionHash": actionHash,
		"nonce":              big.NewInt(int64(nonce)),
	}

	return signUserSignedAction(
		envelope,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "multiSigActionHash", Type: "bytes32"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:SendMultiSig",
		privateKey,
	)
}

func signUserSignedAction(
	action map[string]any,
	payloadTypes []apitypes.Type,
	primaryType string,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	typedData := userSignedPayload(
		primaryType,
		payloadTypes,
		action,
	)

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return signature{}, fmt.Errorf(
			"failed generating hash for typed data: %w",
			err,
		)
	}

	return signHash(common.BytesToHash(hash), privateKey)
}

func signUsdTransferAction(
	action usdTransferAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"destination":      action.Destination,
		"amount":           action.Amount,
		"time":             big.NewInt(action.Time),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "destination", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "time", Type: "uint64"},
		},
		"HyperliquidTransaction:UsdSend",
		privateKey,
	)
}

func signSpotTransferAction(
	action spotTransferAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"destination":      action.Destination,
		"token":            action.Token,
		"amount":           action.Amount,
		"time":             big.NewInt(action.Time),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "destination", Type: "string"},
			{Name: "token", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "time", Type: "uint64"},
		},
		"HyperliquidTransaction:SpotSend",
		privateKey,
	)
}

func signWithdrawFromBridgeAction(
	action withdrawFromBridgeAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"destination":      action.Destination,
		"amount":           action.Amount,
		"time":             big.NewInt(action.Time),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "destination", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "time", Type: "uint64"},
		},
		"HyperliquidTransaction:Withdraw",
		privateKey,
	)
}

func signUsdClassTransferAction(
	action usdClassTransferAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"amount":           action.Amount,
		"toPerp":           action.ToPerp,
		"nonce":            big.NewInt(action.Nonce),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "toPerp", Type: "bool"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:UsdClassTransfer",
		privateKey,
	)
}

func signSendAssetAction(
	action sendAssetAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"destination":      action.Destination,
		"sourceDex":        action.SourceDex,
		"destinationDex":   action.DestinationDex,
		"token":            action.Token,
		"amount":           action.Amount,
		"fromSubAccount":   action.FromSubAccount,
		"nonce":            big.NewInt(action.Nonce),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "destination", Type: "string"},
			{Name: "sourceDex", Type: "string"},
			{Name: "destinationDex", Type: "string"},
			{Name: "token", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "fromSubAccount", Type: "string"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:SendAsset",
		privateKey,
	)
}

func signUserDexAbstractionAction(
	action map[string]any,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	return signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "user", Type: "address"},
			{Name: "enabled", Type: "bool"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:UserDexAbstraction",
		privateKey,
	)
}

func signConvertToMultiSigUserAction(
	action convertToMultiSigUserAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"signers":          action.Signers,
		"nonce":            big.NewInt(action.Nonce),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "signers", Type: "string"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:ConvertToMultiSigUser",
		privateKey,
	)
}

func signTokenDelegateAction(
	action tokenDelegateAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"validator":        action.Validator,
		"wei":              big.NewInt(action.Wei),
		"isUndelegate":     action.IsUndelegate,
		"nonce":            big.NewInt(action.Nonce),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "validator", Type: "address"},
			{Name: "wei", Type: "uint64"},
			{Name: "isUndelegate", Type: "bool"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:TokenDelegate",
		privateKey,
	)
}

func signAgentAction(
	action approveAgentAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"agentAddress":     action.AgentAddress,
		"agentName":        action.AgentName,
		"nonce":            big.NewInt(action.Nonce),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "agentAddress", Type: "address"},
			{Name: "agentName", Type: "string"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:ApproveAgent",
		privateKey,
	)
}

func signApproveBuilderFeeAction(
	action approveBuilderFeeAction,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	actionMap := map[string]any{
		"hyperliquidChain": action.HyperliquidChain,
		"maxFeeRate":       action.MaxFeeRate,
		"builder":          action.Builder,
		"nonce":            big.NewInt(action.Nonce),
	}

	return signUserSignedAction(
		actionMap,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "maxFeeRate", Type: "string"},
			{Name: "builder", Type: "address"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:ApproveBuilderFee",
		privateKey,
	)
}

// hashAction creates a Keccak256 hash of the action following the Hyperliquid
// protocol
func hashAction[T any](
	action T,
	vaultAddress mo.Option[common.Address],
	nonce uint64,
	expiresAfter mo.Option[time.Duration],
) (common.Hash, error) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	enc.SetCustomStructTag("json")
	enc.UseCompactInts(true)

	if err := enc.Encode(action); err != nil {
		return common.Hash{}, fmt.Errorf(
			"failed to msgpack-encode action: %w",
			err,
		)
	}

	data := buf.Bytes()
	fmt.Println("msg pack", hexutil.Encode(data))

	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, nonce)
	data = append(data, nonceBytes...)

	if v, ok := vaultAddress.Get(); ok {
		data = append(data, 0x01)
		data = append(data, v.Bytes()...)
	} else {
		data = append(data, 0x00)
	}

	if e, ok := expiresAfter.Get(); ok {
		data = append(data, 0x00)
		eBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(eBytes, uint64(e.Milliseconds()))
		data = append(data, eBytes...)
	}

	return crypto.Keccak256Hash(data), nil
}

// signHash signs a hash using the private key and returns
// a signature
func signHash(
	hash common.Hash,
	privateKey *ecdsa.PrivateKey,
) (signature, error) {
	var out signature

	// Sign the hash
	sig, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return out, fmt.Errorf("failed to sign: %w", err)
	}

	if len(sig) != 65 {
		return out, fmt.Errorf("invalid signature length: %d", len(sig))
	}

	// sig = [R || S || V]
	copy(out.R[:], sig[:32])
	copy(out.S[:], sig[32:64])
	v := sig[64]

	// Ethereum canonical V = 27 or 28
	if v < 27 {
		v += 27
	}

	out.V = v

	return out, nil
}

func constructPhantomAgent(
	hash common.Hash,
	isMainnet bool,
) apitypes.TypedDataMessage {
	var source string
	if isMainnet {
		source = "a"
	} else {
		source = "b"
	}

	return apitypes.TypedDataMessage{
		"source":       source,
		"connectionId": hash,
	}
}

func l1Payload(
	phantomAgent apitypes.TypedDataMessage,
) apitypes.TypedData {
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Agent": {
				{Name: "source", Type: "string"},
				{Name: "connectionId", Type: "bytes32"},
			},
		},
		PrimaryType: "Agent",
		Domain: apitypes.TypedDataDomain{
			Name:              "Exchange",
			Version:           "1",
			ChainId:           math.NewHexOrDecimal256(1337),
			VerifyingContract: "0x0000000000000000000000000000000000000000",
		},
		Message: phantomAgent,
	}
}

func userSignedPayload(
	primaryType string,
	payloadTypes []apitypes.Type,
	action apitypes.TypedDataMessage,
) apitypes.TypedData {
	types := apitypes.Types{
		"EIP712Domain": {
			{Name: "name", Type: "string"},
			{Name: "version", Type: "string"},
			{Name: "chainId", Type: "uint256"},
			{Name: "verifyingContract", Type: "address"},
		},
	}

	types[primaryType] = payloadTypes

	return apitypes.TypedData{
		Types:       types,
		PrimaryType: primaryType,
		Domain: apitypes.TypedDataDomain{
			Name:              "HyperliquidSignTransaction",
			Version:           "1",
			ChainId:           math.NewHexOrDecimal256(421614),
			VerifyingContract: "0x0000000000000000000000000000000000000000",
		},
		Message: action,
	}
}
