package payload

import (
	"fmt"
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

// MSISDN retrievs phone number
func (tx *Transaction) MSISDN() string {
	for _, v := range tx.Result.ResultParameters.ResultParameter {
		if v.Key == "ReceiverPartyPublicName" && v.Value != "" {
			vals := strings.Split(fmt.Sprint(v.Value), " - ")
			return strings.TrimSpace(vals[0])
		}
	}
	return ""
}

// TransactionReceipt retrievs phone number
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

// TransactionAmount retrievs phone number
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
	return time.Now()
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
