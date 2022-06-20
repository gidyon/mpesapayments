package payload

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Transaction is response from mpesa
type Transaction struct {
	Result struct {
		ResultType               int    `json:"ResultType"`
		ResultCode               int    `json:"ResultCode"`
		ResultDesc               string `json:"ResultDesc"`
		OriginatorConversationID string `json:"OriginatorConversationID"`
		ConversationID           string `json:"ConversationID"`
		TransactionID            string `json:"TransactionID"`
		ResultParameters         struct {
			ResultParameter []struct {
				Key   string      `json:"Key"`
				Value interface{} `json:"Value"`
			} `json:"ResultParameter"`
		} `json:"ResultParameters"`
		ReferenceData struct {
			ReferenceItem struct {
				Key   string      `json:"Key"`
				Value interface{} `json:"Value"`
			} `json:"ReferenceItem"`
		} `json:"ReferenceData"`
	} `json:"Result"`
}

// Succeeded checks whether transaction was successful
func (tx *Transaction) Succeeded() bool {
	return tx.Result.ResultCode == 0
}

// OriginatorConversationID is the originator id
func (tx *Transaction) OriginatorConversationID() string {
	return tx.Result.OriginatorConversationID
}

// ConversationID returns value of tx.Result.ConversationID
func (tx *Transaction) ConversationID() string {
	return tx.Result.ConversationID
}

func (tx *Transaction) MSISDN() string {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "ReceiverPartyPublicName" && v.Value != "" {
			vals := strings.Split(fmt.Sprint(v.Value), " - ")
			return strings.TrimSpace(vals[0])
		}
	}
	return ""
}

// TransactionReceipt retrieves transaction receipt
func (tx *Transaction) TransactionReceipt() string {
	if tx.Result.TransactionID != "" {
		return tx.Result.TransactionID
	}
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "TransactionReceipt" && v.Value != "" {
			return strings.TrimSpace(fmt.Sprint(v.Value))
		}
	}
	return ""
}

// TransactionAmount retrievs transaction phone number
func (tx *Transaction) TransactionAmount() float64 {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "TransactionAmount" {
			val, ok := v.Value.(float64)
			if ok {
				return val
			}
		}
	}
	return 0
}

// B2CRecipientIsRegisteredCustomer checks whether the b2c recipient is a registred customer
func (tx *Transaction) B2CRecipientIsRegisteredCustomer() bool {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "B2CRecipientIsRegisteredCustomer" && v.Value != "" {
			return strings.TrimSpace(fmt.Sprint(v.Value)) == "Y"
		}
	}
	return false
}

// B2CWorkingAccountAvailableFunds returns the available funds for the organization
func (tx *Transaction) B2CWorkingAccountAvailableFunds() float64 {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "B2CWorkingAccountAvailableFunds" {
			val, ok := v.Value.(float64)
			if ok {
				return val
			}
		}
	}
	return 0
}

// B2CUtilityAccountAvailableFunds returns the utility funds available
func (tx *Transaction) B2CUtilityAccountAvailableFunds() float64 {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "B2CUtilityAccountAvailableFunds" {
			val, ok := v.Value.(float64)
			if ok {
				return val
			}
		}
	}
	return 0
}

// B2CChargesPaidAccountAvailableFunds ...
func (tx *Transaction) B2CChargesPaidAccountAvailableFunds() float64 {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "B2CChargesPaidAccountAvailableFunds" {
			val, ok := v.Value.(float64)
			if ok {
				return val
			}
		}
	}
	return 0
}

func getTransactionTimev2(transactionTimeStr string) (time.Time, error) {
	// 21.08.2017 12:01:59
	if len(transactionTimeStr) != 19 {
		return time.Now(), nil
	}

	timeRFC3339Str := fmt.Sprintf(
		"%s-%s-%sT%s:%s:%s+00:00",
		transactionTimeStr[6:10],  // year
		transactionTimeStr[3:5],   // month
		transactionTimeStr[:2],    // day
		transactionTimeStr[11:13], // hour
		transactionTimeStr[14:16], // minutes
		transactionTimeStr[17:19], // seconds
	)

	return time.Parse(time.RFC3339, timeRFC3339Str)
}

// TransactionCompletedDateTime is time the transacction was completed
func (tx *Transaction) TransactionCompletedDateTime() time.Time {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "TransactionCompletedDateTime" && v.Value != "" {
			t, err := getTransactionTimev2(fmt.Sprint(v.Value))
			if err == nil {
				return t
			}
		}
	}
	return time.Now().UTC()
}

// ReceiverPartyPublicName ...
func (tx *Transaction) ReceiverPartyPublicName() string {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "ReceiverPartyPublicName" && v.Value != "" {
			vals := strings.Split(fmt.Sprint(v.Value), " - ")
			if len(vals) > 1 {
				return strings.TrimSpace(vals[1])
			}
			return strings.TrimSpace(vals[0])
		}
	}
	return ""
}

// QueueTimeoutURL ...
func (tx *Transaction) QueueTimeoutURL() string {
	v, ok := tx.Result.ReferenceData.ReferenceItem.Value.(string)
	if ok {
		return v
	}
	return fmt.Sprint(tx.Result.ReferenceData.ReferenceItem.Value)
}

// IncomingTransactionOnfon is the payload for incoming transaction from onfon
type IncomingTransactionOnfon struct {
	OriginatorConversationID         string `json:"originatorConversationID"`
	ResultType                       string `json:"resultType"`
	ResultCode                       string `json:"resultCode"`
	ResultDesc                       string `json:"resultDesc"`
	ConversationID                   string `json:"conversationID"`
	TransactionAmount                string `json:"transactionAmount"`
	TransactionID                    string `json:"transactionID"`
	TransactionReceipt               string `json:"transactionReceipt"`
	ReceiverPartyPublicName          string `json:"receiverPartyPublicName"`
	B2CUtilityAccountAvailableFunds  string `json:"b2CUtilityAccountAvailableFunds"`
	TransactionCompletedDateTime     string `json:"transactionCompletedDateTime"`
	B2CRecipientIsRegisteredCustomer string `json:"b2CRecipientIsRegisteredCustomer"`
}

// MSISDN retrieves phone number
func (tx *IncomingTransactionOnfon) MSISDN() string {
	vals := strings.Split(fmt.Sprint(tx.ReceiverPartyPublicName), " - ")
	if len(vals) == 1 {
		vals = strings.Split(fmt.Sprint(tx.ReceiverPartyPublicName), " ")
	}
	return strings.TrimSpace(vals[0])
}

// CompletedDateTime is time the transacction was completed
func (tx *IncomingTransactionOnfon) CompletedDateTime() time.Time {
	t, err := getTransactionTimev2(fmt.Sprint(tx.TransactionCompletedDateTime))
	if err == nil {
		return t
	}
	return time.Now()
}

// B2CUtilityAccountAvailableFundsV2 retrievs B2CUtilityAccountAvailableFunds as float64
func (tx *IncomingTransactionOnfon) B2CUtilityAccountAvailableFundsV2() float64 {
	val, err := strconv.ParseFloat(tx.B2CUtilityAccountAvailableFunds, 64)
	if err == nil {
		return val
	}
	return 0
}

// Amount retrievs transaction amount
func (tx *IncomingTransactionOnfon) Amount() float64 {
	val, err := strconv.ParseFloat(tx.TransactionAmount, 64)
	if err == nil {
		return val
	}
	return 0
}

// B2CRecipientIsRegisteredCustomerV2 checks whether the b2c recipient is a registred customer
func (tx *IncomingTransactionOnfon) B2CRecipientIsRegisteredCustomerV2() bool {
	return tx.B2CRecipientIsRegisteredCustomer == "Y"
}

// Succeeded checks whether transaction was successful
func (tx *IncomingTransactionOnfon) Succeeded() bool {
	return tx.ResultCode == "0"
}
