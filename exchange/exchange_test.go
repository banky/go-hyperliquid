package exchange

// func TestOrder(t *testing.T) {
// 	privateKey, err := crypto.GenerateKey()
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	e, err := New(Config{
// 		BaseURL: constants.TESTNET_API_URL,
// 		// BaseURL:    constants.MAINNET_API_URL,
// 		SkipWS:     true,
// 		PrivateKey: privateKey,
// 	})
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	r, err := e.Order(
// 		context.Background(),
// 		"BTC",
// 		true,
// 		10,
// 		10,
// 		OrderType{Limit: &LimitOrder{Tif: "Ioc"}},
// 	)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	fmt.Println(r)

// }
