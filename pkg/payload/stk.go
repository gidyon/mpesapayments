package payload

import (
	"fmt"
	"time"
)

// STKPayload is incoming transaction payload for stk push
type STKPayload struct {
	Body Body `json:"Body,omitempty"`
}

// Body ...
type Body struct {
	STKCallback STKCallback `json:"stkCallback,omitempty"`
}

// STKCallback ...
type STKCallback struct {
	MerchantRequestID string       `json:"MerchantRequestID,omitempty"`
	CheckoutRequestID string       `json:"CheckoutRequestID,omitempty"`
	ResultCode        int          `json:"ResultCode,omitempty"`
	ResultDesc        string       `json:"ResultDesc,omitempty"`
	CallbackMetadata  CallbackMeta `json:"CallbackMetadata,omitempty"`
}

// CallbackMeta is response body for successful response
type CallbackMeta struct {
	Item []Item `json:"Item,omitempty"`
}

// Item ...
type Item struct {
	Name  string      `json:"Name,omitempty"`
	Value interface{} `json:"Value,omitempty"`
}

// GetAmount returns the transaction amount
func (c *CallbackMeta) GetAmount() float32 {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return 0
	}

	v, ok := c.Item[0].Value.(float64)
	if !ok {
		return 0
	}

	return float32(v)
}

// MpesaReceiptNumber returns the receipt number
func (c *CallbackMeta) MpesaReceiptNumber() string {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return ""
	}

	return fmt.Sprint(c.Item[1].Value)
}

// Balance returns the transaction balance
func (c *CallbackMeta) Balance() string {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return ""
	}

	return fmt.Sprint(c.Item[2].Value)
}

// GetTransTime returns the transaction time
func (c *CallbackMeta) GetTransTime() time.Time {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return time.Now().UTC()
	}

	var (
		t   time.Time
		err error
	)

	switch itemsLen {
	case 4:
		t, err = getTransactionTime(fmt.Sprint(c.Item[2].Value))
		if err != nil {
			t = time.Now().UTC()
		}
	case 5:
		t, err = getTransactionTime(fmt.Sprint(c.Item[3].Value))
		if err != nil {
			t = time.Now().UTC()
		}
	}

	return t
}

// PhoneNumber returns the phone number
func (c *CallbackMeta) PhoneNumber() string {
	itemsLen := len(c.Item)
	if itemsLen != 5 && itemsLen != 4 {
		return ""
	}

	var (
		v  float64
		ok bool
	)

	switch itemsLen {
	case 4:
		v, ok = c.Item[3].Value.(float64)
		if !ok {
			return fmt.Sprint(c.Item[3].Value)
		}
	case 5:
		v, ok = c.Item[4].Value.(float64)
		if !ok {
			return fmt.Sprint(c.Item[4].Value)
		}
	}

	return fmt.Sprintf("%.0f", v)
}

func getTransactionTime(transactionTimeStr string) (time.Time, error) {
	// 20200816204116
	if len(transactionTimeStr) != 14 {
		return time.Now(), nil
	}

	timeRFC3339Str := fmt.Sprintf(
		"%s-%s-%sT%s:%s:%s+00:00",
		transactionTimeStr[:4],    // year
		transactionTimeStr[4:6],   // month
		transactionTimeStr[6:8],   // day
		transactionTimeStr[8:10],  // hour
		transactionTimeStr[10:12], // minutes
		transactionTimeStr[12:],   // seconds
	)

	return time.Parse(time.RFC3339, timeRFC3339Str)
}

// {
// 	"Body":
// 	{"stkCallback":
// 	 {
// 	  "MerchantRequestID": "21605-295434-4",
// 	  "CheckoutRequestID": "ws_CO_04112017184930742",
// 	  "ResultCode": 0,
// 	  "ResultDesc": "The service request is processed successfully.",
// 	  "CallbackMetadata":
// 	   {
// 		"Item":
// 		[
// 		{
// 		  "Name": "Amount",
// 		  "Value": 1
// 		},
// 		{
// 		  "Name": "MpesaReceiptNumber",
// 		  "Value": "LK451H35OP"
// 		},
// 		{
// 		  "Name": "Balance"
// 		},
// 		{
// 		  "Name": "TransactionDate",
// 		  "Value": 20171104184944
// 		 },
// 		{
// 		  "Name": "PhoneNumber",
// 		  "Value": 254727894083
// 		}
// 		]
// 	   }
// 	 }
// 	}
//    }

type QueryStkRequest struct {
	BusinessShortCode string `json:"BusinessShortCode"`
	Password          string `json:"Password"`
	Timestamp         string `json:"Timestamp"`
	CheckoutRequestID string `json:"CheckoutRequestID"`
}

type QueryStkResponse struct {
	ResponseCode        string `json:"ResponseCode"`
	ResponseDescription string `json:"ResponseDescription"`
	MerchantRequestID   string `json:"MerchantRequestID"`
	CheckoutRequestID   string `json:"CheckoutRequestID"`
	ResultCode          string `json:"ResultCode"`
	ResultDesc          string `json:"ResultDesc"`
}
