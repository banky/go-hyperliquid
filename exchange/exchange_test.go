package exchange

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/banky/go-hyperliquid/constants"
	"github.com/banky/go-hyperliquid/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/joho/godotenv"
)

func TestOrder(t *testing.T) {
	_ = godotenv.Load("../.env")

	privateKey, err := crypto.HexToECDSA(os.Getenv("WALLET_PRIVATE_KEY"))
	if err != nil {
		log.Fatalf("invalid private key: %v", err)
	}

	e, err := New(Config{
		BaseURL: constants.TESTNET_API_URL,
		// BaseURL:    constants.MAINNET_API_URL,
		SkipWS:     true,
		PrivateKey: privateKey,
	})
	if err != nil {
		t.Fatal(err)
	}

	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	userState, err := e.info.UserState(
		context.Background(),
		address,
		"",
	)

	fmt.Println("user state:", userState)

	// Place an order that should rest by setting the price very low
	orderResponse, err := e.Order(
		context.Background(),
		NewOrderRequest(
			"ETH",
			true,
			0.2,
			1100,
			WithLimitOrder(LimitOrder{Tif: "Gtc"}),
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	oid := orderResponse.Resting.Oid

	cancelResponse, err := e.Cancel(
		context.Background(),
		oid,
		"ETH",
	)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(cancelResponse)
}

func TestModify(t *testing.T) {
	_ = godotenv.Load("../.env")

	privateKey, err := crypto.HexToECDSA(os.Getenv("WALLET_PRIVATE_KEY"))
	if err != nil {
		log.Fatalf("invalid private key: %v", err)
	}

	e, err := New(Config{
		BaseURL: constants.TESTNET_API_URL,
		// BaseURL:    constants.MAINNET_API_URL,
		SkipWS:     true,
		PrivateKey: privateKey,
	})
	if err != nil {
		t.Fatal(err)
	}

	cloid := types.HexToCloid("0x00000000000000000000000000000001")

	orderResponse, err := e.Order(
		context.Background(),
		NewOrderRequest(
			"ETH",
			true,
			0.2,
			1100,
			WithLimitOrder(LimitOrder{Tif: "Gtc"}),
			WithCloid(cloid),
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if orderResponse.Resting == nil {
		t.Fatal("resting is nil")
	}

	oid := orderResponse.Resting.Oid

	modifyResponse, err := e.ModifyOrder(
		context.Background(),
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
	if err != nil {
		t.Fatal(err)
	}

	newOid := modifyResponse.Resting.Oid

	fmt.Println(modifyResponse)

	_, err = e.Cancel(
		context.Background(),
		newOid,
		"ETH",
	)
	if err != nil {
		t.Fatal(err)
	}
}
