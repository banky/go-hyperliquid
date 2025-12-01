package exchange

import (
	"bytes"
	"encoding/binary"
	"fmt"
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
	e *Exchange,
	action T,
	nonce uint64,
) (signature, error) {
	actionHash, err := hashAction(
		action,
		e.vaultAddress,
		nonce,
		e.expiresAfter,
	)
	if err != nil {
		return signature{}, fmt.Errorf("failed to create action hash: %w", err)
	}

	phantomAgent := constructPhantomAgent(actionHash, e.rest.IsMainnet())
	typedData := l1Payload(phantomAgent)

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return signature{}, fmt.Errorf(
			"failed generating hash for typed data: %w",
			err,
		)
	}

	return e.signHash(common.BytesToHash(hash))
}

// signL1ActionWithVault signs an L1 action with an optional vault address
// override
func (e *Exchange) signL1ActionWithVault(
	action map[string]any,
	nonce uint64,
	vaultAddress mo.Option[common.Address],
) (signature, error) {
	actionHash, err := hashAction(
		action,
		vaultAddress,
		nonce,
		e.expiresAfter,
	)
	if err != nil {
		return signature{}, fmt.Errorf("failed to create action hash: %w", err)
	}

	phantomAgent := constructPhantomAgent(actionHash, e.rest.IsMainnet())
	typedData := l1Payload(phantomAgent)

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return signature{}, fmt.Errorf(
			"failed generating hash for typed data: %w",
			err,
		)
	}

	return e.signHash(common.BytesToHash(hash))
}

// signMultiSigAction signs a multi-signature action
func (e *Exchange) signMultiSigAction(
	action map[string]any,
	vaultAddress *common.Address,
	nonce uint64,
) (signature, error) {
	var vaultOpt mo.Option[common.Address]
	if vaultAddress != nil {
		vaultOpt = mo.Some(*vaultAddress)
	}

	actionHash, err := hashAction(
		action,
		vaultOpt,
		nonce,
		e.expiresAfter,
	)
	if err != nil {
		return signature{}, fmt.Errorf("failed to create action hash: %w", err)
	}

	phantomAgent := constructPhantomAgent(actionHash, e.rest.IsMainnet())
	typedData := l1Payload(phantomAgent)

	hash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return signature{}, fmt.Errorf(
			"failed generating hash for typed data: %w",
			err,
		)
	}

	return e.signHash(common.BytesToHash(hash))
}

func (e *Exchange) signUserSignedAction(
	action map[string]any,
	payloadTypes []apitypes.Type,
	primaryType string,
) (signature, error) {
	var hyperliquidChain = "Mainnet"
	if !e.rest.IsMainnet() {
		hyperliquidChain = "Testnet"
	}

	action["hyperliquidChain"] = hyperliquidChain

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

	return e.signHash(common.BytesToHash(hash))
}

func (e *Exchange) signUsdTransferAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "destination", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "time", Type: "uint64"},
		},
		"HyperliquidTransaction:UsdSend",
	)
}

func (e *Exchange) signSpotTransferAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "destination", Type: "string"},
			{Name: "token", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "time", Type: "uint64"},
		},
		"HyperliquidTransaction:SpotSend",
	)
}

func (e *Exchange) signWithdrawFromBridgeAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "destination", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "time", Type: "uint64"},
		},
		"HyperliquidTransaction:Withdraw",
	)
}

func (e *Exchange) signUsdClassTransferAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "amount", Type: "string"},
			{Name: "toPerp", Type: "bool"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:UsdClassTransfer",
	)
}

func (e *Exchange) signSendAssetAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
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
	)
}

func (e *Exchange) signUserDexAbstractionAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "user", Type: "address"},
			{Name: "enabled", Type: "bool"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:UserDexAbstraction",
	)
}

func (e *Exchange) signConvertToMultiSigUserAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "signers", Type: "string"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:ConvertToMultiSigUser",
	)
}

func (e *Exchange) signTokenDelegateAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "validator", Type: "address"},
			{Name: "wei", Type: "uint64"},
			{Name: "isUndelegate", Type: "bool"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:TokenDelegate",
	)
}

func (e *Exchange) signAgentAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "agentAddress", Type: "address"},
			{Name: "agentName", Type: "string"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:ApproveAgent",
	)
}

func (e *Exchange) signApproveBuilderFeeAction(
	action map[string]any,
) (signature, error) {
	return e.signUserSignedAction(
		action,
		[]apitypes.Type{
			{Name: "hyperliquidChain", Type: "string"},
			{Name: "maxFeeRate", Type: "string"},
			{Name: "builder", Type: "address"},
			{Name: "nonce", Type: "uint64"},
		},
		"HyperliquidTransaction:ApproveBuilderFee",
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
func (e *Exchange) signHash(hash common.Hash) (signature, error) {
	var out signature

	// Sign the hash
	sig, err := crypto.Sign(hash.Bytes(), e.privateKey)
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
