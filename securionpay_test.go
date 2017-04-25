package securionpay_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/orijtech/securionpay"
)

func TestAddCardRequest(t *testing.T) {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		t.Fatalf("initializing client from env: %v", err)
	}

	tests := [...]struct {
		acr *securionpay.AddCardRequest

		wantErr bool
	}{
		0: {
			acr:     nil,
			wantErr: true,
		},

		1: {
			acr: &securionpay.AddCardRequest{
				CustomerID: customerID1,

				Card: cardFromFile("./testdata/addcard1.json"),
			},
		},
	}

	cRTripper := &customRoundTripper{route: addCardRoute}
	client.SetHTTPRoundTripper(cRTripper)

	for i, tt := range tests {
		card, err := client.AddCard(tt.acr)
		if tt.wantErr {
			if err == nil {
				t.Errorf("#%d: expected an error", i)
			}
			continue
		}

		if err != nil {
			t.Errorf("#%d: err: %v", i, err)
			continue
		}

		if card == nil || card.ID == "" {
			t.Errorf("#%d: expected a non-blank card")
		}
	}
}

func TestCharge(t *testing.T) {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		t.Fatalf("initializing client from env: %v", err)
	}

	tests := [...]struct {
		charge *securionpay.Charge

		wantErr bool
	}{
		0: {
			charge:  nil,
			wantErr: true,
		},

		1: {
			charge: &securionpay.Charge{
				Card: cardFromFile("./testdata/addcard1.json"),
			},
		},
	}

	cRTripper := &customRoundTripper{route: chargeRoute}
	client.SetHTTPRoundTripper(cRTripper)

	for i, tt := range tests {
		card, err := client.Charge(tt.charge)
		if tt.wantErr {
			if err == nil {
				t.Errorf("#%d: expected an error", i)
			}
			continue
		}

		if err != nil {
			t.Errorf("#%d: err: %v", i, err)
			continue
		}

		if card == nil || card.ID == "" {
			t.Errorf("#%d: expected a non-blank card")
		}
	}
}

const (
	// Test keys
	customerID1 = "customerID1"

	// routes
	chargeRoute  = "/charge"
	addCardRoute = "/addcard"
)

var knownTestKeys = map[string]bool{
	customerID1: true,
}

func knownCustomerID(id string) bool {
	_, known := knownTestKeys[id]
	return known
}

type customRoundTripper struct{ route string }

var _ http.RoundTripper = (*customRoundTripper)(nil)

var (
	noAuthResponse    = makeResp("expecting the API key in the basic auth", http.StatusForbidden)
	noCardResponse    = makeResp("expecting a card", http.StatusBadRequest)
	invalidCustomerID = makeResp("no customerID was passed in", http.StatusBadRequest)

	noPasswordExpectedResponse = makeResp("no password was expected, please check the docs", http.StatusForbidden)
)

func makeResp(status string, statusCode int) *http.Response {
	return &http.Response{
		Status:     status,
		StatusCode: statusCode,
		Header:     make(http.Header),
	}
}

func (ct *customRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Firstly check the basic Auth
	username, password, ok := req.BasicAuth()
	if !ok || username == "" {
		return noAuthResponse, nil
	}
	if password != "" {
		return noPasswordExpectedResponse, nil
	}

	switch ct.route {
	case chargeRoute:
		return ct.chargeRoundTrip(req)
	case addCardRoute:
		return ct.addCardRoundTrip(req)
	default:
		return makeResp(fmt.Sprintf("%q unknown route", ct.route), http.StatusNotFound), nil
	}
}

var (
	blankCardOrCustomerIDResp = makeResp("either `customerId` or `card` must be set", http.StatusBadRequest)
	noChargeResponse          = makeResp("no charge was passed in", http.StatusBadRequest)
)

func (ct *customRoundTripper) chargeRoundTrip(req *http.Request) (*http.Response, error) {
	slurp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	charge := new(securionpay.Charge)
	blankCharge := *charge
	if err := json.Unmarshal(slurp, charge); err != nil {
		return nil, err
	}
	if blankCharge == *charge {
		return noChargeResponse, nil
	}

	// Either customerID or card have to be set
	blankCustomerID := charge.CustomerID == ""
	blankCard := charge.Card == nil || charge.Card == ""
	if blankCard && blankCustomerID {
		return blankCardOrCustomerIDResp, nil
	}

	f, err := os.Open("testdata/chargeResp1.json")
	if err != nil {
		return makeResp(err.Error(), http.StatusInternalServerError), nil
	}

	prc, pwc := io.Pipe()
	okResp := makeResp("200 OK", http.StatusOK)
	okResp.Body = prc
	go func() {
		defer f.Close()
		defer pwc.Close()
		io.Copy(pwc, f)
	}()

	return okResp, nil
}

func (ct *customRoundTripper) addCardRoundTrip(req *http.Request) (*http.Response, error) {
	// From path, split out the first part
	splits := strings.Split(req.URL.Path, "/")
	if len(splits) < 2 || !knownCustomerID(splits[len(splits)-2]) {
		return invalidCustomerID, nil
	}

	slurp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	card := new(securionpay.Card)
	blankCard := *card
	if err := json.Unmarshal(slurp, card); err != nil {
		return nil, err
	}
	if blankCard == *card {
		return noCardResponse, nil
	}

	f, err := os.Open("testdata/addcard1.json")
	if err != nil {
		return makeResp(err.Error(), http.StatusInternalServerError), nil
	}

	prc, pwc := io.Pipe()
	okResp := makeResp("200 OK", http.StatusOK)
	okResp.Body = prc
	go func() {
		defer f.Close()
		defer pwc.Close()
		io.Copy(pwc, f)
	}()

	return okResp, nil
}

func cardFromFile(path string) *securionpay.Card {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	blob, err := ioutil.ReadAll(f)

	if err != nil {
		return nil
	}
	card := new(securionpay.Card)
	if err := json.Unmarshal(blob, card); err != nil {
		return nil
	}
	return card
}
