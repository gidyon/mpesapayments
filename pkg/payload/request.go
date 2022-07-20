package payload

// MpesaPayload is incoming paybill transaction from mpesa
type MpesaPayload struct {
	TransactionType   string `json:"transaction_type,omitempty"`
	TransID           string `json:"trans_id,omitempty"`
	TransTime         string `json:"trans_time,omitempty"`
	TransAmount       string `json:"trans_amount,omitempty"`
	BusinessShortCode string `json:"business_short_code,omitempty"`
	BillRefNumber     string `json:"bill_ref_number,omitempty"`
	InvoiceNumber     string `json:"invoice_number,omitempty"`
	OrgAccountBalance string `json:"org_account_balance,omitempty"`
	ThirdPartyTransID string `json:"third_party_trans_id,omitempty"`
	MSISDN            string `json:"msisdn,omitempty"`
	FirstName         string `json:"first_name,omitempty"`
	MiddleName        string `json:"middle_name,omitempty"`
	LastName          string `json:"last_name,omitempty"`
}

// MpesaPayloadV2 is incoming paybill transaction from mpesa
type MpesaPayloadV2 struct {
	TransactionType   string `json:"TransactionType"`
	TransID           string `json:"TransID"`
	TransTime         string `json:"TransTime"`
	TransAmount       string `json:"TransAmount"`
	BusinessShortCode string `json:"BusinessShortCode"`
	BillRefNumber     string `json:"BillRefNumber"`
	InvoiceNumber     string `json:"InvoiceNumber"`
	OrgAccountBalance string `json:"OrgAccountBalance"`
	ThirdPartyTransID string `json:"ThirdPartyTransID"`
	MSISDN            string `json:"MSISDN"`
	FirstName         string `json:"FirstName"`
	MiddleName        string `json:"MiddleName"`
	LastName          string `json:"LastName"`
}
