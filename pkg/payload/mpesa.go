package payload

// MpesaPayload is incoming paybill transaction from mpesa
type MpesaPayload struct {
	TransactionType   string
	TransID           string
	TransTime         string
	TransAmount       string
	BusinessShortCode string
	BillRefNumber     string
	InvoiceNumber     string
	OrgAccountBalance string
	ThirdPartyTransID string
	MSISDN            string
	FirstName         string
	MiddleName        string
	LastName          string
}
