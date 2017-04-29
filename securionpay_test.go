package securionpay_test

import (
	"bytes"
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

func blobify(v interface{}) []byte {
	blob, _ := json.Marshal(v)
	return blob
}

func TestCreateToken(t *testing.T) {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		t.Fatalf("initializing client from env: %v", err)
	}

	tests := [...]struct {
		tReq    *securionpay.TokenRequest
		wantErr bool
		comment string
	}{
		0: {tReq: tokenReqByIDFromFile(tokenReqID1)},
		1: {tReq: tokenReqByIDFromFile(tokenReqNoCVC), wantErr: true, comment: "no CVC"},

		2: {tReq: nil, wantErr: true},
	}

	cRTripper := &customRoundTripper{route: createTokenRoute}
	client.SetHTTPRoundTripper(cRTripper)

	for i, tt := range tests {
		tok, err := client.NewToken(tt.tReq)
		if tt.wantErr {
			if err == nil {
				t.Errorf("#%d: want non-nil error", i)
			}
			continue
		}

		if err != nil {
			t.Errorf("#%d gotErr=%q", i, err)
			continue
		}

		if tok == nil {
			t.Errorf("#%d got a nil token", i)
		}
	}
}

func TestRetrieveToken(t *testing.T) {
	client, err := securionpay.NewClientFromEnv()
	if err != nil {
		t.Fatalf("initializing client from env: %v", err)
	}

	tests := [...]struct {
		tokenID   string
		wantErr   bool
		wantToken *securionpay.Token
	}{
		0: {tokenID: "unknownID", wantErr: true},
		1: {tokenID: tokenID1, wantToken: _tokenByIDFromFile(tokenID1)},
		2: {tokenID: tokenID2, wantToken: _tokenByIDFromFile(tokenID2)},
		3: {tokenID: "", wantErr: true},
		4: {tokenID: "     ", wantErr: true},
	}

	cRTripper := &customRoundTripper{route: retrieveTokenRoute}
	client.SetHTTPRoundTripper(cRTripper)

	for i, tt := range tests {
		tok, err := client.FindTokenByID(tt.tokenID)
		if tt.wantErr {
			if err == nil {
				t.Errorf("#%d: want non-nil error", i)
			}
			continue
		}

		if err != nil {
			t.Errorf("#%d gotErr=%q", i, err)
			continue
		}

		gotBlob := blobify(tok)
		wantBlob := blobify(tt.wantToken)

		if !bytes.Equal(gotBlob, wantBlob) {
			t.Errorf("#%d\ngot:  %s\nwant: %s", i, gotBlob, wantBlob)
		}
	}
}

const (
	// Test keys
	customerID1   = "customerID1"
	tokenID1      = "tokenID1"
	tokenID2      = "tokenID2"
	tokenReqID1   = "id1"
	tokenReqNoCVC = "no-cvc"

	// routes
	chargeRoute        = "/charge"
	addCardRoute       = "/addcard"
	retrieveTokenRoute = "/retrieve-token"
	createTokenRoute   = "/create-token"
)

var knownTestKeys = map[string]bool{
	customerID1: true,
}

func knownCustomerID(id string) bool {
	_, known := knownTestKeys[id]
	return known
}

var knownTokenKeys = map[string]bool{
	tokenID1: true,
	tokenID2: true,
}

func knownTokenID(id string) bool {
	_, known := knownTokenKeys[id]
	return known
}

type customRoundTripper struct{ route string }

var _ http.RoundTripper = (*customRoundTripper)(nil)

var (
	noAuthResponse    = makeResp("expecting the API key in the basic auth", http.StatusForbidden)
	noCardResponse    = makeResp("expecting a card", http.StatusBadRequest)
	invalidCustomerID = makeResp("no customerID was passed in", http.StatusBadRequest)
	invalidTokenID    = makeResp("invalid tokenID", http.StatusBadRequest)

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
	case retrieveTokenRoute:
		return ct.retrieveTokenRoundTrip(req)
	case createTokenRoute:
		return ct.createTokenRoundTrip(req)
	default:
		return makeResp(fmt.Sprintf("%q unknown route", ct.route), http.StatusNotFound), nil
	}
}

var (
	blankCardOrCustomerIDResp = makeResp("either `customerId` or `card` must be set", http.StatusBadRequest)
	noChargeResponse          = makeResp("no charge was passed in", http.StatusBadRequest)
)

func (ct *customRoundTripper) retrieveTokenRoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != "GET" {
		return makeResp("only GET allowed", http.StatusMethodNotAllowed), nil
	}
	splits := strings.Split(req.URL.Path, "/")
	if len(splits) < 1 {
		return invalidTokenID, nil
	}

	tokenID := splits[len(splits)-1]
	if !knownTokenID(tokenID) {
		return invalidTokenID, nil
	}

	token, err := tokenByIDFromFile(tokenID)
	if err != nil {
		return makeResp(err.Error(), http.StatusInternalServerError), nil
	}

	blob, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}

	prc, pwc := io.Pipe()
	okResp := makeResp("200 OK", http.StatusOK)
	okResp.Body = prc
	go func() {
		defer pwc.Close()
		pwc.Write(blob)
	}()

	return okResp, nil
}

func (ct *customRoundTripper) createTokenRoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != "POST" {
		return makeResp("only \"POST\" allowed", http.StatusMethodNotAllowed), nil
	}

	slurp, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return makeResp(err.Error(), http.StatusBadRequest), nil
	}

	tokReq := new(securionpay.TokenRequest)
	blankTokReq := *tokReq
	if err := json.Unmarshal(slurp, tokReq); err != nil {
		return makeResp(err.Error(), http.StatusBadRequest), nil
	}
	if blankTokReq == *tokReq {
		return makeResp("expecting a token request", http.StatusBadRequest), nil
	}

	tok, err := tokenByIDFromFile(tokenID1)
	if err != nil {
		return makeResp(err.Error(), http.StatusInternalServerError), nil
	}
	blob, _ := json.Marshal(tok)

	prc, pwc := io.Pipe()
	okResp := makeResp("200 OK", http.StatusOK)
	okResp.Body = prc
	go func() {
		defer pwc.Close()
		pwc.Write(blob)
	}()

	return okResp, nil

}

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

func retrFromFile(path string, save interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	blob, err := ioutil.ReadAll(f)
	if err != nil {
		return nil
	}

	return json.Unmarshal(blob, save)
}

func cardFromFile(path string) *securionpay.Card {
	saveCard := new(securionpay.Card)
	if err := retrFromFile(path, saveCard); err != nil {
		return nil
	}
	return saveCard
}

func _tokenByIDFromFile(tokenID string) *securionpay.Token {
	tok, _ := tokenByIDFromFile(tokenID)
	return tok
}

func tokenByIDFromFile(tokenID string) (*securionpay.Token, error) {
	fullTokenPath := fmt.Sprintf("./testdata/token-%s", tokenID)
	return tokenFromFile(fullTokenPath)
}

func tokenFromFile(path string) (*securionpay.Token, error) {
	saveToken := new(securionpay.Token)
	if err := retrFromFile(path, saveToken); err != nil {
		return nil, err
	}
	return saveToken, nil
}

func tokenReqByIDFromFile(id string) *securionpay.TokenRequest {
	fullPath := fmt.Sprintf("./testdata/token-req-%s", id)
	saveTokenReq := new(securionpay.TokenRequest)
	if err := retrFromFile(fullPath, saveTokenReq); err != nil {
		return nil
	}
	return saveTokenReq
}
