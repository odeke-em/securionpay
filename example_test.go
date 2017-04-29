package securionpay_test

import (
	"fmt"
	"log"

	"github.com/orijtech/securionpay"
)

func Example_client_AddCard() {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	card, err := client.AddCard(&securionpay.AddCardRequest{
		CustomerID: "johnDoeCustomerID",
		Card: &securionpay.Card{
			ID:    "card_8P7OWXA5xiTS1ISnyZcum1KV",
			Brand: "Visa",

			CreatedAt:   1415810511,
			FingerPrint: "e3d8suyIDgFg3pE7",
			ExpiryMonth: 11,
			ExpiryYear:  2022,
			CustomerID:  "cust_AoR0wvgntQWRUYMdZNLYMz5R",

			First6Digits: "424242",
			Last4Digits:  "4242",

			CardHolderName: "John Doe",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("newly registered card: %#v\n", card)
}

func Example_client_Charge() {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	charge, err := client.Charge(&securionpay.Charge{
		Description: "Lunch reimbursement",
		Currency:    securionpay.USD,

		AmountMinorCurrencyUnits: 1500, // Amount of $15 in cents

		Billing: &securionpay.Billing{
			Address: &securionpay.Address{
				Country: "USA",
				City:    "Washington",
				State:   "Washington DC",
				Line1:   "1600 Pennsylvania Ave NW",
				Zip:     "20500",
			},
		},

		Card: &securionpay.Card{
			ID:    "card_8P7OWXA5xiTS1ISnyZcum1KV",
			Brand: "Visa",

			CreatedAt:   1415810511,
			FingerPrint: "e3d8suyIDgFg3pE7",
			ExpiryMonth: 11,
			ExpiryYear:  2022,
			CustomerID:  "cust_AoR0wvgntQWRUYMdZNLYMz5R",

			First6Digits: "424242",
			Last4Digits:  "4242",

			CardHolderName: "John Doe",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("charge processed: %#v\n", charge)
}

func Example_client_fromEnv() {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("client: %#v\n", client)
}

func Example_client_findTokenByID() {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	token, err := client.FindTokenByID("tok_NGsyDoJQXop5Pqqi6HizbJTe")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("my token: %#v\n", token)
}

func Example_client_newToken() {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	token, err := client.NewToken(&securionpay.TokenRequest{
		CardNumber:     "24242424242424",
		ExpiryMonth:    10,
		ExpiryYear:     2020,
		SecurityCode:   "123",
		CardHolderName: "Ashley Jones",
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("newly created token: %#v\n", token)
}
