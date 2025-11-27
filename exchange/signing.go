package exchange

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/samber/mo"
	"github.com/vmihailenco/msgpack/v5"
)

// signL1Action signs an action using EIP-712 typed data signing
// This implements the L1 action signing mechanism used by Hyperliquid
func (e *Exchange) signL1Action(
	action map[string]any,
	nonce uint64,
) (signature, error) {
	actionHash, err := e.hashAction(
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

// func (e *Exchange) signUserSignedAction(
// 	action map[string]any,
// 	// payload
// )

// hashAction creates a Keccak256 hash of the action following the Hyperliquid protocol
func (e *Exchange) hashAction(
	action map[string]any,
	vaultAddress mo.Option[common.Address],
	nonce uint64,
	expiresAfter mo.Option[time.Duration],
) (common.Hash, error) {
	data, err := msgpack.Marshal(action)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to marshal action: %w", err)
	}

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
