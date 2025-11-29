package exchange

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"testing"

	"github.com/banky/go-hyperliquid/constants"
	"github.com/banky/go-hyperliquid/types"
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
		NewOrderRequest(
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
		oid,
		"ETH",
	)
	require.CmpNoError(err)

	fmt.Println(cancelResponse)
}

func (s *ExchangeIntegrationSuite) TestModify(assert, require *td.T) {
	ctx := context.Background()

	cloid := types.HexToCloid("0x00000000000000000000000000000001")

	orderResponse, err := s.exchange.Order(
		ctx,
		NewOrderRequest(
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
		NewModifyRequest(
			NewOrderRequest(
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
		newOid,
		"ETH",
	)
	require.CmpNoError(err)
}
