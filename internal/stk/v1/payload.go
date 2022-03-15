package stk

// STKRequestBody is STK push request payload
type STKRequestBody struct {
	BusinessShortCode string `json:"BusinessShortCode,omitempty"`
	Password          string `json:"Password,omitempty"`
	Timestamp         string `json:"Timestamp,omitempty"`
	TransactionType   string `json:"TransactionType,omitempty"`
	Amount            string `json:"Amount,omitempty"`
	PartyA            string `json:"PartyA,omitempty"`
	PartyB            string `json:"PartyB,omitempty"`
	PhoneNumber       string `json:"PhoneNumber,omitempty"`
	CallBackURL       string `json:"CallBackURL,omitempty"`
	AccountReference  string `json:"AccountReference,omitempty"`
	TransactionDesc   string `json:"TransactionDesc,omitempty"`
}
