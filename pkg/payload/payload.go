package payload

// AccountBalanceRequest is request to make balance inquiry
type AccountBalanceRequest struct {
	CommandID          string `json:"CommandID,omitempty"`
	PartyA             int32  `json:"PartyA,omitempty"`
	IdentifierType     int32  `json:"IdentifierType,omitempty"`
	Remarks            string `json:"Remarks,omitempty"`
	Initiator          string `json:"Initiator,omitempty"`
	SecurityCredential string `json:"SecurityCredential,omitempty"`
	QueueTimeOutURL    string `json:"QueueTimeOutURL,omitempty"`
	ResultURL          string `json:"ResultURL,omitempty"`
}

// B2CRequest is request to transact between an M-Pesa short code to a phone number registered on M-Pesa.
type B2CRequest struct {
	InitiatorName      string `json:"InitiatorName,omitempty"`
	SecurityCredential string `json:"SecurityCredential,omitempty"`
	CommandID          string `json:"CommandID,omitempty"`
	Amount             string `json:"Amount,omitempty"`
	PartyA             string `json:"PartyA,omitempty"`
	PartyB             int64  `json:"PartyB,omitempty"`
	Remarks            string `json:"Remarks,omitempty"`
	QueueTimeOutURL    string `json:"QueueTimeOutURL,omitempty"`
	ResultURL          string `json:"ResultURL,omitempty"`
	Occassion          string `json:"Occassion,omitempty"`
}

// ReversalRequest is request to reverses a M-Pesa transaction.
type ReversalRequest struct {
	CommandID              string `json:"CommandID,omitempty"`
	ReceiverParty          int32  `json:"ReceiverParty,omitempty"`
	ReceiverIdentifierType int64  `json:"ReceiverIdentifierType,omitempty"`
	Remarks                string `json:"Remarks,omitempty"`
	Initiator              string `json:"Initiator,omitempty"`
	SecurityCredential     string `json:"SecurityCredential,omitempty"`
	QueueTimeOutURL        string `json:"QueueTimeOutURL,omitempty"`
	ResultURL              string `json:"ResultURL,omitempty"`
	TransactionID          string `json:"TransactionID,omitempty"`
	Occassion              string `json:"Occassion,omitempty"`
}
