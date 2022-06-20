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
