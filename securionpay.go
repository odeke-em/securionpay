package securionpay

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/orijtech/otils"
)

type Client struct {
	sync.RWMutex

	apiKey string

	rt http.RoundTripper
}

const (
	envAPIKeyKey = "SECURIONPAY_API_KEY"
)

var errEmptyAPIKey = fmt.Errorf("missing API key, please set %q in your environment", envAPIKeyKey)

func NewClientFromEnv() (*Client, error) {
	retrAPIKey := os.Getenv(envAPIKeyKey)
	if retrAPIKey == "" {
		return nil, errEmptyAPIKey
	}
	client := &Client{apiKey: retrAPIKey}
	return client, nil
}

func (c *Client) SetHTTPRoundTripper(rt http.RoundTripper) {
	c.Lock()
	c.rt = rt
	c.Unlock()
}

func (c *Client) SetAPIKey(key string) {
	c.Lock()
	c.apiKey = key
	c.Unlock()
}

type CardType string

const (
	CreditCardType CardType = "Credit Card"
)

type Brand string

const (
	BrandVisa       Brand = "Visa"
	BrandAMEX       Brand = "American Express"
	BrandMasterCard Brand = "MasterCard"
	BrandDiscover   Brand = "Discover"
	BrandJCB        Brand = "JBC"
	BrandDinersClub Brand = "Diners Club"
	BrandUnknown    Brand = "Unknown"
)

type ObjectType string

var _ json.Marshaler = (*ObjectType)(nil)

func (ot *ObjectType) MarshalJSON() ([]byte, error) {
	str := "card"
	if ot != nil {
		stripped := strings.TrimSpace(string(*ot))
		if stripped != "" {
			str = stripped
		}
	}

	quoted := strconv.Quote(str)
	return []byte(quoted), nil
}

type Card struct {
	ID             string     `json:"id"`
	ObjectType     ObjectType `json:"objectType"`
	CreatedAt      int64      `json:"created"`
	First6Digits   string     `json:"first6"`
	Last4Digits    string     `json:"last4"`
	FingerPrint    string     `json:"fingerprint"`
	ExpiryMonth    int        `json:"expMonth,string"`
	ExpiryYear     int        `json:"expYear,string"`
	CardHolderName string     `json:"cardholderName"`
	CustomerID     string     `json:"customerId"`
	Brand          string     `json:"brand"`
	Type           CardType   `json:"type"`
	Country        string     `json:"addressCountry,omitempty"`
	City           string     `json:"addressCity,omitempty"`
	State          string     `json:"addressState,omitempty"`
	ZIP            string     `json:"addressZip,omitempty"`
	AddressLine1   string     `json:"addressLine1,omitempty"`
	AddressLine2   string     `json:"addressLine2,omitempty"`

	FraudCheckData *FraudCheckData `json:"fraudCheckData"`
}

type FraudCheckData struct {
	IPAddress      string `json:"ipAddress,omitempty"`
	IPCountry      string `json:"ipCountry,omitempty"`
	Email          string `json:"email,omitempty"`
	UserAgent      string `json:"userAgent,omitempty"`
	AcceptLanguage string `json:"acceptLanguage"`
}

type Customer struct {
	ID string `json:"id"`
}

type AddCardRequest struct {
	CustomerID string `json:"customerId"`
	Card       *Card  `json:"card"`
}

var (
	errInvalidCustomerID = errors.New("invalid customerID")

	errBlankCard    = errors.New("expecting a non-blank card")
	errUnsetCardID  = errors.New("expecting the card ID to have been set")
	errBlankTokenID = errors.New("expecting a non-blank token ID")

	errBlankAddCardRequest = errors.New("expecting a non-blank card request")
)

func (c *Card) Validate() error {
	if c == nil {
		return errBlankCard
	}
	if strings.TrimSpace(c.ID) == "" {
		return errUnsetCardID
	}

	return nil
}

const addCardEndpointURL = "https://api.securionpay.com/customers/{{.CustomerID}}/cards"

var addCardEndpointTmpl = template.Must(template.New("addCard").Parse(addCardEndpointURL))

func (acr *AddCardRequest) generateURL() (string, error) {
	buf := new(bytes.Buffer)
	if err := addCardEndpointTmpl.Execute(buf, acr); err != nil {
		return "", err
	}
	return string(buf.Bytes()), nil
}

func (c *Client) AddCard(acr *AddCardRequest) (*Card, error) {
	if acr == nil {
		return nil, errBlankAddCardRequest
	}

	card := acr.Card
	if err := card.Validate(); err != nil {
		return nil, err
	}

	customerID := strings.TrimSpace(acr.CustomerID)
	if customerID == "" {
		return nil, errUnsetCardID
	}

	endpointURL, err := acr.generateURL()
	if err != nil {
		return nil, err
	}

	blob, err := json.Marshal(card)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", endpointURL, bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}

	blob, err = c.doAuthThenReqAndSlurpResponse(req)
	if err != nil {
		return nil, err
	}

	registeredCard := new(Card)
	if err := json.Unmarshal(blob, registeredCard); err != nil {
		return nil, err
	}

	return registeredCard, nil
}

func (c *Client) httpRoundTripper() http.RoundTripper {
	c.RLock()
	rt := c.rt
	c.RUnlock()

	if rt == nil {
		rt = http.DefaultTransport
	}

	return rt
}

func (c *Client) httpClient() *http.Client {
	return &http.Client{Transport: c.httpRoundTripper()}
}

func (c *Client) _apiKey() string {
	c.RLock()
	key := c.apiKey
	c.RUnlock()

	return key
}

type Currency string

const (
	USD   Currency = "USD"
	Euros Currency = "EUR"
	CAD   Currency = "CAD"
)

type Charge struct {
	// AmountMinorCurrencyUnits is the charge in minor
	// amounts of currency. For example 10€ is represented
	// as "1000" and 10¥ is represented as "10"
	AmountMinorCurrencyUnits int `json:"amount,string"`

	// Currency is the 3 digit ISO currency code
	// for example: EUR, USD, CAD
	Currency    Currency `json:"currency"`
	Description string   `json:"description"`

	// Card can either be:
	// a) card token
	// b) card details
	// c) card identifier
	Card interface{} `json:"card,omitempty"`

	// Either CustomerID or Card can be set
	CustomerID string `json:"customerId,omitempty"`

	Shipping *Shipping `json:"shipping,omitempty"`
	Billing  *Billing  `json:"billing,omitempty"`

	Captured bool `json:"captured,omitempty"`
}

type Address struct {
	Zip     string `json:"zip"`
	Line1   string `json:"line1"`
	Line2   string `json:"line2"`
	City    string `json:"city"`
	State   string `json:"state"`
	Country string `json:"country"`
}

type Shipping struct {
	Name    string   `json:"name"`
	Address *Address `json:"address"`
}

type Billing struct {
	Address *Address `json:"address"`

	// VAT is the tax identification number
	VAT string `json:"vat"`
}

type ChargeResponse struct {
	ID          string     `json:"id"`
	Amount      float32    `json:"amount"`
	Currency    Currency   `json:"currency"`
	CreatedAt   int64      `json:"created"`
	ObjectType  ObjectType `json:"objectType"`
	Description string     `json:"description"`

	Card *Card `json:"card"`

	Captured bool `json:"captured"`
	Refunded bool `json:"refunded"`
	Disputed bool `json:"disputed"`

	Refunds  []*Refund  `json:"refunds,omitempty"`
	Disputes []*Dispute `json:"dispute,omitempty"`
}

type Refund *Charge

type Dispute struct {
	ObjectType string `json:"objectType"`
	CreatedAt  int64  `json:"created"`
	UpdatedAt  int64  `json:"updated"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	Amount     int    `json:"amount"`

	AcceptedAsLost bool `json:"acceptedAsLost"`
	// Currency is the 3 digit ISO currency code
	// for example: EUR, USD, CAD
	Currency Currency `json:"currency"`
}

type DisputeStatus string

var (
	errBlankCharge = errors.New("expecting a non-blank charge")

	errEitherBlankCardOrCustomerIDMustBeSet = errors.New("either `customerId` or `card` must be set")
)

func (creq *Charge) Validate() error {
	if creq == nil {
		return errBlankCharge
	}
	// The rule is that either customerId or card have to be set
	blankCard := creq.Card == nil || creq.Card == ""
	blankCustomerID := creq.CustomerID == ""
	if blankCard && blankCustomerID {
		return errEitherBlankCardOrCustomerIDMustBeSet
	}
	return nil
}

const chargeEndpointURL = "https://api.securionpay.com/charges"

func (c *Client) Charge(creq *Charge) (*ChargeResponse, error) {
	if err := creq.Validate(); err != nil {
		return nil, err
	}

	blob, err := json.Marshal(creq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", chargeEndpointURL, bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}

	blob, err = c.doAuthThenReqAndSlurpResponse(req)
	if err != nil {
		return nil, err
	}

	cResp := new(ChargeResponse)
	if err := json.Unmarshal(blob, cResp); err != nil {
		return nil, err
	}

	return cResp, nil
}

type Token struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created"`

	ObjectType     ObjectType `json:"objectType"`
	First6Digits   string     `json:"first6"`
	Last4Digits    string     `json:"last4"`
	FingerPrint    string     `json:"fingerprint"`
	ExpiryMonth    int        `json:"expMonth,string"`
	ExpiryYear     int        `json:"expYear,string"`
	Brand          string     `json:"brand"`
	Type           CardType   `json:"type"`
	CardHolderName string     `json:"cardholderName"`

	AddressLine1 string `json:"addressLine1,omitempty"`
	AddressLine2 string `json:"addressLine2,omitempty"`
	City         string `json:"addressCity,omitempty"`
	State        string `json:"addressState,omitempty"`
	ZIP          string `json:"addressZip,omitempty"`
	Country      string `json:"addressCountry,omitempty"`

	Used bool  `json:"used,omitempty"`
	Card *Card `json:"card"`

	FraudCheckData   *FraudCheckData   `json:"fraudCheckData,omitempty"`
	ThreeDSecureInfo *ThreeDSecureInfo `json:"threeDSecureInfo,omitempty"`
}

type ThreeDSecureInfo struct {
	// AmountMinorCurrencyUnits is the charge in minor
	// amounts of currency. For example 10€ is represented
	// as "1000" and 10¥ is represented as "10"
	AmountMinorCurrencyUnits int `json:"amount,string"`

	// Currency is the 3 digit ISO currency code
	// for example: EUR, USD, CAD
	Currency Currency `json:"currency"`

	Enrolled       bool           `json:"enrolled,omitempty"`
	LiabilityShift LiabilityShift `json:"liabilityShift,omitempty"`
}

type LiabilityShift string

const (
	SuccessfulShift LiabilityShift = "successful"
	FailedShift     LiabilityShift = "failed"
	NotPossible     LiabilityShift = "not_possible"
)

type TokenRequest struct {
	CardNumber  string `json:"number"`
	ExpiryMonth int    `json:"expMonth,string"`
	ExpiryYear  int    `json:"expYear,string"`

	SecurityCode   string `json:"cvc"`
	CardHolderName string `json:"cardholderName"`
	City           string `json:"addressCity,omitempty"`
	State          string `json:"addressState,omitempty"`
	ZIP            string `json:"addressZip,omitempty"`
	AddressLine1   string `json:"addressLine1,omitempty"`
	AddressLine2   string `json:"addressLine2,omitempty"`
	Country        string `json:"addressCountry,omitempty"`

	FraudCheckData *FraudCheckData `json:"fraudCheckData"`
}

var (
	errNilTokenRequest   = errors.New("nil token request passed in")
	errEmptySecurityCode = errors.New("expecting a non-empty security code aka \"cvc\"")
)

func (treq *TokenRequest) Validate() error {
	if treq == nil {
		return errNilTokenRequest
	}
	if strings.TrimSpace(treq.SecurityCode) == "" {
		return errEmptySecurityCode
	}
	return nil
}

func (c *Client) NewToken(treq *TokenRequest) (*Token, error) {
	if err := treq.Validate(); err != nil {
		return nil, err
	}

	blob, err := json.Marshal(treq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", tokensEndpointURL, bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}

	blob, err = c.doAuthThenReqAndSlurpResponse(req)
	if err != nil {
		return nil, err
	}

	tok := new(Token)
	if err := json.Unmarshal(blob, tok); err != nil {
		return nil, err
	}

	return tok, nil
}

const tokensEndpointURL = "https://api.securionpay.com/tokens"

// GET https://api.securionpay.com/tokens/{TOKEN_ID}
func (c *Client) FindTokenByID(tokenID string) (*Token, error) {
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, errBlankTokenID
	}

	fullURL := fmt.Sprintf("%s/%s", tokensEndpointURL, tokenID)
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	blob, err := c.doAuthThenReqAndSlurpResponse(req)
	if err != nil {
		return nil, err
	}

	tok := new(Token)
	if err := json.Unmarshal(blob, tok); err != nil {
		return nil, err
	}
	return tok, nil
}

func (c *Client) doAuthThenReqAndSlurpResponse(req *http.Request) ([]byte, error) {
	req.SetBasicAuth(c._apiKey(), "")
	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if res.Body != nil {
		defer res.Body.Close()
	}

	if !otils.StatusOK(res.StatusCode) {
		errMsg := res.Status
		if res.Body != nil {
			slurp, _ := ioutil.ReadAll(res.Body)
			if len(slurp) > 0 {
				errMsg = string(slurp)
			}
		}
		return nil, errors.New(errMsg)
	}

	return ioutil.ReadAll(res.Body)
}
