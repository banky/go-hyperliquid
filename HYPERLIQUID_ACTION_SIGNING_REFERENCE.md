# Hyperliquid Action Signing Reference

This document categorizes all Hyperliquid exchange actions by their signing method.
There are two primary signing methods: **L1 Actions (Phantom Agent)** and **User-Signed Actions (EIP-712)**.

---

## L1 Actions (Phantom Agent Signing)

These actions use `sign_l1_action()` which:
- Creates a hash of the action using msgpack
- Constructs a "phantom agent" from the hash
- Signs using EIP-712 with domain name "Exchange" and chainId 1337
- Used for trading and most account management operations

### Trading Actions
1. **order** - Place orders
   - Type: `order`
   - Function: `bulk_orders()`

2. **batchModify** - Modify existing orders
   - Type: `batchModify`
   - Function: `bulk_modify_orders_new()`

3. **cancel** - Cancel orders by order ID
   - Type: `cancel`
   - Function: `bulk_cancel()`

4. **cancelByCloid** - Cancel orders by client order ID
   - Type: `cancelByCloid`
   - Function: `bulk_cancel_by_cloid()`

5. **scheduleCancel** - Schedule cancellation of all orders
   - Type: `scheduleCancel`
   - Function: `schedule_cancel()`

### Account Management Actions
6. **updateLeverage** - Update leverage for an asset
   - Type: `updateLeverage`
   - Function: `update_leverage()`

7. **updateIsolatedMargin** - Update isolated margin
   - Type: `updateIsolatedMargin`
   - Function: `update_isolated_margin()`

8. **setReferrer** - Set referrer code
   - Type: `setReferrer`
   - Function: `set_referrer()`

9. **createSubAccount** - Create a sub-account
   - Type: `createSubAccount`
   - Function: `create_sub_account()`

10. **subAccountTransfer** - Transfer between sub-accounts
    - Type: `subAccountTransfer`
    - Function: `sub_account_transfer()`

11. **subAccountSpotTransfer** - Transfer spot assets between sub-accounts
    - Type: `subAccountSpotTransfer`
    - Function: `sub_account_spot_transfer()`

12. **vaultTransfer** - Transfer to/from vault
    - Type: `vaultTransfer`
    - Function: `vault_usd_transfer()`

### Deployment Actions (Spot)
13. **spotDeploy** - Various spot deployment actions
    - Type: `spotDeploy` (with various subtypes)
    - Functions:
      - `spot_deploy_register_token()`
      - `spot_deploy_user_genesis()`
      - `spot_deploy_freeze_user()`
      - `spot_deploy_enable_freeze_privilege()`
      - `spot_deploy_revoke_freeze_privilege()`
      - `spot_deploy_enable_quote_token()`
      - `spot_deploy_genesis()`
      - `spot_deploy_register_spot()`
      - `spot_deploy_register_hyperliquidity()`
      - `spot_deploy_set_deployer_trading_fee_share()`

### Deployment Actions (Perp)
14. **perpDeploy** - Perpetual deployment actions
    - Type: `perpDeploy`
    - Functions:
      - `perp_deploy_register_asset()`
      - `perp_deploy_set_oracle()`

### Validator/Signer Actions
15. **CSignerAction** - C signer actions
    - Type: `CSignerAction`
    - Functions:
      - `c_signer_unjail_self()`
      - `c_signer_jail_self()`

16. **CValidatorAction** - C validator actions
    - Type: `CValidatorAction`
    - Functions:
      - `c_validator_register()`
      - `c_validator_change_profile()`
      - `c_validator_unregister()`

### Other L1 Actions
17. **evmUserModify** - EVM user modifications
    - Type: `evmUserModify`
    - Function: `use_big_blocks()`

18. **agentEnableDexAbstraction** - Enable DEX abstraction for agent
    - Type: `agentEnableDexAbstraction`
    - Function: `agent_enable_dex_abstraction()`

19. **noop** - No operation (for testing/debugging)
    - Type: `noop`
    - Function: `noop()`

---

## User-Signed Actions (EIP-712 Direct Signing)

These actions use `sign_user_signed_action()` which:
- Adds `signatureChainId` (0x66eee) and `hyperliquidChain` (Mainnet/Testnet) to action
- Signs using EIP-712 with domain name "HyperliquidSignTransaction" and chainId 421614
- Used for transfers and special account operations

### Transfer Actions
1. **usdSend** - Transfer USD between accounts
   - Type: `usdSend`
   - Primary Type: `HyperliquidTransaction:UsdSend`
   - Function: `usd_transfer()`
   - Sign Types: `USD_SEND_SIGN_TYPES`
   - Fields: `hyperliquidChain`, `destination`, `amount`, `time`

2. **spotSend** - Transfer spot tokens between accounts
   - Type: `spotSend`
   - Primary Type: `HyperliquidTransaction:SpotSend`
   - Function: `spot_transfer()`
   - Sign Types: `SPOT_TRANSFER_SIGN_TYPES`
   - Fields: `hyperliquidChain`, `destination`, `token`, `amount`, `time`

3. **withdraw3** - Withdraw from bridge
   - Type: `withdraw3`
   - Primary Type: `HyperliquidTransaction:Withdraw`
   - Function: `withdraw_from_bridge()`
   - Sign Types: `WITHDRAW_SIGN_TYPES`
   - Fields: `hyperliquidChain`, `destination`, `amount`, `time`

4. **usdClassTransfer** - Transfer between USD classes (perp/spot)
   - Type: `usdClassTransfer`
   - Primary Type: `HyperliquidTransaction:UsdClassTransfer`
   - Function: `usd_class_transfer()`
   - Sign Types: `USD_CLASS_TRANSFER_SIGN_TYPES`
   - Fields: `hyperliquidChain`, `amount`, `toPerp`, `nonce`

5. **sendAsset** - Send assets between DEXs/accounts
   - Type: `sendAsset`
   - Primary Type: `HyperliquidTransaction:SendAsset`
   - Function: `send_asset()`
   - Sign Types: `SEND_ASSET_SIGN_TYPES`
   - Fields: `hyperliquidChain`, `destination`, `sourceDex`, `destinationDex`, `token`, `amount`, `fromSubAccount`, `nonce`

### Special Account Actions
6. **approveAgent** - Approve an agent
   - Type: `approveAgent`
   - Primary Type: `HyperliquidTransaction:ApproveAgent`
   - Function: `approve_agent()`
   - Fields: `hyperliquidChain`, `agentAddress`, `agentName`, `nonce`

7. **approveBuilderFee** - Approve builder fee
   - Type: `approveBuilderFee`
   - Primary Type: `HyperliquidTransaction:ApproveBuilderFee`
   - Function: `approve_builder_fee()`
   - Fields: `hyperliquidChain`, `maxFeeRate`, `builder`, `nonce`

8. **convertToMultiSigUser** - Convert account to multi-sig
   - Type: `convertToMultiSigUser`
   - Primary Type: `HyperliquidTransaction:ConvertToMultiSigUser`
   - Function: `convert_to_multi_sig_user()`
   - Sign Types: `CONVERT_TO_MULTI_SIG_USER_SIGN_TYPES`
   - Fields: `hyperliquidChain`, `signers`, `nonce`

9. **tokenDelegate** - Delegate tokens to validator
   - Type: `tokenDelegate`
   - Primary Type: `HyperliquidTransaction:TokenDelegate`
   - Function: `token_delegate()`
   - Sign Types: `TOKEN_DELEGATE_TYPES`
   - Fields: `hyperliquidChain`, `validator`, `wei`, `isUndelegate`, `nonce`

10. **userDexAbstraction** - Enable/disable DEX abstraction for user
    - Type: `userDexAbstraction`
    - Primary Type: `HyperliquidTransaction:UserDexAbstraction`
    - Function: `user_dex_abstraction()`
    - Sign Types: `USER_DEX_ABSTRACTION_SIGN_TYPES`
    - Fields: `hyperliquidChain`, `user`, `enabled`, `nonce`

---

## Multi-Signature Actions

### For L1 Actions (when used in multisig context)
Use `sign_multi_sig_l1_action_payload()`:
- Wraps action in envelope: `[multiSigUser.lower(), outerSigner.lower(), action]`
- Signs the envelope using L1 phantom agent signing
- Used for: order, cancel, modify, updateLeverage, etc. (all L1 actions)

**Implementation in Go:**
```go
func signMultisigL1ActionPayload[T any](
    action T,
    nonce uint64,
    privateKey *ecdsa.PrivateKey,
    vaultAddress mo.Option[common.Address],
    expiresAfter mo.Option[time.Duration],
    isMainnet bool,
    multiSigUser common.Address,
    outerSigner common.Address,
) (signature, error)
```

### For User-Signed Actions (when used in multisig context)
Use `sign_multi_sig_user_signed_action_payload()`:
- Adds `payloadMultiSigUser` and `outerSigner` fields directly to action
- Adds corresponding type definitions after `hyperliquidChain`
- Signs using user-signed action path (EIP-712)
- Used for: usdSend, spotSend, sendAsset, convertToMultiSigUser, etc. (all user-signed actions)

**Implementation in Go (NEEDS TO BE IMPLEMENTED):**
```go
func signMultiSigUserSignedActionPayload(
    action map[string]any,
    privateKey *ecdsa.PrivateKey,
    payloadTypes []apitypes.Type,
    primaryType string,
    isMainnet bool,
    multiSigUser common.Address,
    outerSigner common.Address,
) (signature, error) {
    // 1. Add multisig fields to action
    action["payloadMultiSigUser"] = strings.ToLower(multiSigUser.Hex())
    action["outerSigner"] = strings.ToLower(outerSigner.Hex())

    // 2. Add multisig types (insert after hyperliquidChain)
    enrichedTypes := addMultiSigTypes(payloadTypes)

    // 3. Sign using user-signed action path
    return signUserSignedAction(action, enrichedTypes, primaryType, privateKey)
}

func addMultiSigTypes(signTypes []apitypes.Type) []apitypes.Type {
    enrichedTypes := []apitypes.Type{}
    for _, signType := range signTypes {
        enrichedTypes = append(enrichedTypes, signType)
        if signType.Name == "hyperliquidChain" {
            enrichedTypes = append(enrichedTypes,
                apitypes.Type{Name: "payloadMultiSigUser", Type: "address"},
                apitypes.Type{Name: "outerSigner", Type: "address"},
            )
        }
    }
    return enrichedTypes
}
```

### Multi-Sig Envelope (outer signature)
After collecting inner signatures, use `sign_multi_sig_action()`:
- Creates hash of multiSigAction (without "type" field)
- Signs envelope with: `{hyperliquidChain, multiSigActionHash, nonce}`
- Primary Type: `HyperliquidTransaction:SendMultiSig`
- This is the SAME for both L1 and User-Signed actions

---

## Summary Table

| Action Type | Signing Method | Multisig Inner Signing | Has signatureChainId | Has hyperliquidChain |
|-------------|----------------|------------------------|----------------------|----------------------|
| L1 Actions | `sign_l1_action` | `sign_multi_sig_l1_action_payload` | No | No |
| User-Signed | `sign_user_signed_action` | `sign_multi_sig_user_signed_action_payload` | Yes | Yes |

---

## Key Differences

### L1 Actions vs User-Signed Actions

**L1 Actions:**
- Domain: "Exchange", chainId: 1337
- Primary Type: "Agent"
- Uses phantom agent construction
- Action is hashed with msgpack
- No signatureChainId or hyperliquidChain in action struct
- Examples: order, cancel, modify, updateLeverage

**User-Signed Actions:**
- Domain: "HyperliquidSignTransaction", chainId: 421614
- Primary Type: varies (e.g., "HyperliquidTransaction:UsdSend")
- Direct EIP-712 signing
- Action includes signatureChainId and hyperliquidChain fields
- Examples: usdSend, spotSend, convertToMultiSigUser

### Multisig Context

**For L1 Actions:**
- Wrap in array envelope: `[multiSigUser, outerSigner, action]`
- Sign the envelope with phantom agent

**For User-Signed Actions:**
- Add fields to action: `action.payloadMultiSigUser` and `action.outerSigner`
- Add corresponding types to sign types array
- Sign the modified action with user-signed method

---

## Implementation Status in Go SDK

### Implemented
- ✅ L1 action signing
- ✅ User-signed action signing
- ✅ L1 multisig payload signing (`signMultisigL1ActionPayload`)
- ✅ Multisig envelope signing (`signMultiSigAction`)

### NOT Implemented (NEEDS IMPLEMENTATION)
- ❌ User-signed multisig payload signing (`signMultiSigUserSignedActionPayload`)
- ❌ `addMultiSigTypes` helper function

This is why `ConvertToMultiSigUser` fails when used in multisig context - it's a user-signed action but the Go SDK only has L1 multisig signing implemented.
