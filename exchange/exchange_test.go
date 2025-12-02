package exchange

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/banky/go-hyperliquid/constants"
	"github.com/banky/go-hyperliquid/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/joho/godotenv"
	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
)

// ExchangeIntegrationSuite groups manual integration tests for the exchange.
type ExchangeIntegrationSuite struct {
	privateKey *ecdsa.PrivateKey
	exchange   *Exchange
}

// Setup is called once before any test runs.
func (s *ExchangeIntegrationSuite) Setup(t *td.T) error {
	_ = godotenv.Load("../.env")

	rawKey := os.Getenv("WALLET_PRIVATE_KEY")
	if rawKey == "" {
		return fmt.Errorf("WALLET_PRIVATE_KEY not set in environment")
	}

	privateKey, err := crypto.HexToECDSA(rawKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	e, err := New(Config{
		BaseURL:    constants.TESTNET_API_URL,
		SkipWS:     true,
		PrivateKey: privateKey,
	})
	if err != nil {
		return fmt.Errorf("failed to create exchange client: %w", err)
	}

	s.privateKey = privateKey
	s.exchange = e

	return nil
}

// Test entry point for the suite.
// By default, the whole suite is skipped unless RUN_EXCHANGE_INTEGRATION=1.
func TestExchangeIntegrationSuite(t *testing.T) {
	_ = godotenv.Load("../.env")
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		fmt.Println("Skipping", os.Getenv("RUN_EXCHANGE_INTEGRATION"))
		t.Skip(
			"skipping ExchangeIntegrationSuite; set RUN_EXCHANGE_INTEGRATION=1 to run",
		)
	}

	tdsuite.Run(t, &ExchangeIntegrationSuite{})
}

func (s *ExchangeIntegrationSuite) TestOrder(assert, require *td.T) {
	ctx := context.Background()

	address := crypto.PubkeyToAddress(s.privateKey.PublicKey)

	userState, err := s.exchange.info.UserState(ctx, address, "")
	require.CmpNoError(err)
	assert.NotNil(userState)

	fmt.Println("user state:", userState)

	// Place an order that should rest by setting the price very low
	orderResponse, err := s.exchange.Order(
		ctx,
		OrderRequest(
			"ETH",
			true,
			0.2,
			1100,
			WithLimitOrder(LimitOrder{Tif: "Gtc"}),
		),
	)
	require.CmpNoError(err)

	// Was: if orderResponse.Resting == nil { t.Fatal("resting is nil") }
	require.NotNil(orderResponse.Resting)
	oid := orderResponse.Resting.Oid

	cancelResponse, err := s.exchange.Cancel(
		ctx,
		CancelRequest("ETH", oid),
	)
	require.CmpNoError(err)

	fmt.Println(cancelResponse)
}

func (s *ExchangeIntegrationSuite) TestOrderWithCloid(assert, require *td.T) {
	ctx := context.Background()

	cloid := types.BigToCloid(big.NewInt(1))

	// Place an order that should rest by setting the price very low
	orderResponse, err := s.exchange.Order(
		ctx,
		OrderRequest(
			"ETH",
			true,
			0.2,
			1100,
			WithLimitOrder(LimitOrder{Tif: "Gtc"}),
			WithCloid(cloid),
		),
	)
	require.CmpNoError(err)
	require.NotNil(orderResponse.Resting)

	cancelResponse, err := s.exchange.CancelByCloid(
		ctx,
		CancelByCloidRequest("ETH", cloid),
	)

	require.NotNil(orderResponse.Resting)

	fmt.Println(cancelResponse)
}

func (s *ExchangeIntegrationSuite) TestModify(assert, require *td.T) {
	ctx := context.Background()

	cloid := types.HexToCloid("0x00000000000000000000000000000001")

	orderResponse, err := s.exchange.Order(
		ctx,
		OrderRequest(
			"ETH",
			true,
			0.2,
			1100,
			WithLimitOrder(LimitOrder{Tif: "Gtc"}),
			WithCloid(cloid),
		),
	)
	require.CmpNoError(err)

	// Was: if orderResponse.Resting == nil { t.Fatal("resting is nil") }
	require.NotNil(orderResponse.Resting)
	oid := orderResponse.Resting.Oid

	modifyResponse, err := s.exchange.ModifyOrder(
		ctx,
		ModifyRequest(
			OrderRequest(
				"ETH",
				true,
				0.1,
				1105,
				WithLimitOrder(LimitOrder{Tif: "Gtc"}),
				WithCloid(cloid),
			),
			WithModifyOrderId(oid),
		),
	)
	require.CmpNoError(err)

	require.NotNil(modifyResponse.Resting)
	newOid := modifyResponse.Resting.Oid

	fmt.Println(modifyResponse)

	_, err = s.exchange.Cancel(
		ctx,
		CancelRequest("ETH", newOid),
	)
	require.CmpNoError(err)
}

func (s *ExchangeIntegrationSuite) TestMarketOrder(assert, require *td.T) {
	ctx := context.Background()

	openResponse, err := s.exchange.MarketOpen(
		ctx,
		MarketOpenRequest(
			"ETH",
			false,
			0.05,
			WithMarketSlippage(0.05),
		),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", openResponse)

	fmt.Println("Waiting to close order")
	time.Sleep(time.Second * 5)
	fmt.Println("Closing order")

	closeResponse, err := s.exchange.MarketClose(
		ctx,
		MarketCloseRequest("ETH"),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", closeResponse)
}

func (s *ExchangeIntegrationSuite) TestScheduleCancel(assert, require *td.T) {
	// Need a lot of volume to test this
	require.TB.Skip()
	ctx := context.Background()

	openResponse, err := s.exchange.Order(
		ctx,
		OrderRequest(
			"ETH",
			true,
			0.2,
			1100,
			WithLimitOrder(LimitOrder{Tif: "Gtc"}),
		),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", openResponse)
	t := time.Now().Add(10 * time.Second)

	cancelResponse, err := s.exchange.ScheduleCancel(
		ctx,
		ScheduleCancelRequest(&t),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", cancelResponse)
}

func (s *ExchangeIntegrationSuite) TestUpdateLeverage(assert, require *td.T) {
	ctx := context.Background()

	response, err := s.exchange.UpdateLeverage(
		ctx,
		UpdateLeverageRequest("ETH", 22),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)
}

func (s *ExchangeIntegrationSuite) TestUpdateIsolatedMargin(
	assert, require *td.T,
) {
	// This test requires an open isolated position
	// Can test by opening position and running
	require.TB.Skip("Skipping test that requires open isolated position")
	ctx := context.Background()

	response, err := s.exchange.UpdateIsolatedMargin(
		ctx,
		UpdateIsolatedMarginRequest("ETH", 1),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)
}

func (s *ExchangeIntegrationSuite) TestSetReferrer(
	assert, require *td.T,
) {
	ctx := context.Background()

	response, err := s.exchange.SetReferrer(
		ctx,
		"ASDFASDF",
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)
}

func (s *ExchangeIntegrationSuite) TestCreateSubAccount(
	assert, require *td.T,
) {
	ctx := context.Background()

	response, err := s.exchange.CreateSubAccount(
		ctx,
		s.getTestAccountName(),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)
}

func (s *ExchangeIntegrationSuite) TestSubAccountTransfer(
	assert, require *td.T,
) {
	ctx := context.Background()

	response, err := s.exchange.CreateSubAccount(
		ctx,
		s.getTestAccountName(),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)

	account := response.Data

	response2, err := s.exchange.SubAccountTransfer(
		ctx,
		account,
		true,
		10_000,
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response2)
}

func (s *ExchangeIntegrationSuite) TestSubAccountSpotTransfer(
	assert, require *td.T,
) {
	ctx := context.Background()

	response, err := s.exchange.CreateSubAccount(
		ctx,
		s.getTestAccountName(),
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)

	account := response.Data

	response2, err := s.exchange.SubAccountSpotTransfer(
		ctx,
		account,
		true,
		"HYPE:0x7317beb7cceed72ef0b346074cc8e7ab",
		1.23,
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response2)
}

func (s *ExchangeIntegrationSuite) TestUsdClassTransfer(
	assert, require *td.T,
) {
	ctx := context.Background()

	response, err := s.exchange.UsdClassTransfer(
		ctx,
		0.1,
		true,
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)
}

func (s *ExchangeIntegrationSuite) TestSendAsset(
	assert, require *td.T,
) {
	ctx := context.Background()

	response, err := s.exchange.SendAsset(
		ctx,
		common.Address{},
		"",
		"test",
		"USDC",
		0.01,
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)
}

func (s *ExchangeIntegrationSuite) TestVaultUsdTransfer(
	assert, require *td.T,
) {
	ctx := context.Background()

	vaultAddress := common.HexToAddress(
		"0xa15099a30bbf2e68942d6f4c43d70d04faeab0a0",
	)

	response, err := s.exchange.VaultUsdTransfer(
		ctx,
		vaultAddress,
		true,
		6_000_000,
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)
}

func (s *ExchangeIntegrationSuite) TestUsdTransfer(
	assert, require *td.T,
) {
	ctx := context.Background()

	destination := common.HexToAddress(
		"0x7851f494001129fcf7adEB85406eE710Dbdb9446",
	)

	response, err := s.exchange.UsdTransfer(
		ctx,
		2.00,
		destination,
	)
	require.CmpNoError(err)

	fmt.Printf("response:%+v\n", response)
}

func (s *ExchangeIntegrationSuite) getTestAccountName() string {
	n := rand.Intn(10000) + 1
	account := fmt.Sprintf("TestAccount%d", n)

	return account
}
