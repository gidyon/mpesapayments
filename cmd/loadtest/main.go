package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/payload"
)

var (
	serverAddress    = flag.String("server-address", "https://localhost:9090", "MPESA Server address")
	stkPath          = flag.String("stk-path", "/api/mpestx/incoming/stkpush", "STK path")
	mpesaPath        = flag.String("mpesa-path", "/api/mpestx/confirmation", "MPESA path")
	parallelRequests = flag.Int("parallel", 10, "Parallel requests to send")
	trips            = flag.Int("trips", 1, "No of trips to make")
	intervalSec      = flag.Int("interval-sec", 0, "Interval in seconds to wait before sending request to gateway")
	disableStk       = flag.Bool("disable-stk", false, "Disables sending load test to stk gateway")
	disableMpesa     = flag.Bool("disable-mpesa", false, "Disables sending load test to mpesa gateway")
	shortCode        = flag.String("short-code", "40853", "MPESA Short Code")
	httpClient       *http.Client
)

func main() {
	flag.Parse()

	fmt.Printf("started sending %d requests\n", *parallelRequests)

	httpClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	var (
		failedRequests = 0
		totalRequests  = 0
		stks           = 0
		lnms           = 0

		wg  = sync.WaitGroup{}
		t1  = time.Now()
		ctx = context.Background()
	)

	for i := 0; i < *trips; i++ {
		if *intervalSec != 0 {
			time.Sleep(time.Duration(*intervalSec) * time.Second)
		}

		for i := 0; i < *parallelRequests; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()

				var (
					r       io.Reader
					address string
				)

				n := randomdata.Number(1, 10)
				if n%2 == 0 && !*disableMpesa {
					r = mpesaPayloadBuffer()
					address = fmt.Sprintf("%s%s", *serverAddress, *mpesaPath)
				} else if !*disableStk {
					r = stkPayloadBuffer()
					address = fmt.Sprintf("%s%s", *serverAddress, *stkPath)
				}

				err := sendRequest(ctx, address, r)
				if err != nil {
					failedRequests++
					fmt.Println(err)
					return
				}

				if n%2 == 0 && !*disableMpesa {
					lnms++
				} else if !*disableStk {
					stks++
				}

				totalRequests++
			}()
		}
	}

	wg.Wait()
	fmt.Printf("\nload test done!! took %f seconds", time.Now().Sub(t1).Seconds())
	fmt.Printf("\nTotalRequests: %d, Failed Requests: %d", totalRequests, failedRequests)
	fmt.Printf("\nSTKs: %d, LNM: %d\n", stks, lnms)
}

func sendRequest(ctx context.Context, url string, body io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("content-type", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer res.Body.Close()

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read response: %v", err)
	}

	response := string(buf)

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %v, response: %v", res.Status, response)
	}

	println(response)

	return nil
}

func txPhoneNumber() int {
	v, err := strconv.Atoi(fmt.Sprintf("2547%d", randomdata.Number(99999999, 1000000000)))
	errs.Panic(err)
	return v
}

func txTime() string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(time.Now().String()[:19], "-", ""), ":", ""), " ", "")
}

func txID() string {
	return strings.ToUpper(randomdata.RandStringRunes(16))
}

// [
// 	BillRefNumber:pwo
// 	BusinessShortCode:4045065
// 	FirstName:GIDEON
// 	InvoiceNumber:
// 	LastName:NG'ANG'A
// 	MSISDN:254716484395
// 	MiddleName:KAMAU
// 	OrgAccountBalance:26.00
// 	ThirdPartyTransID:
// 	TransAmount:2.00
// 	TransID:OIU61LFLVC
// 	TransTime:2020-09-30-17-16-53
// 	TransactionType:Pay Bill
// ]

func mpesaPayloadBuffer() io.Reader {
	mpesaPayload := payload.MpesaPayload{
		TransactionType:   "PAY_BILL",
		TransID:           txID(),
		TransTime:         txTime(),
		TransAmount:       fmt.Sprint(randomdata.Number(10, 1000)),
		BusinessShortCode: *shortCode,
		BillRefNumber:     randomdata.Adjective(),
		InvoiceNumber:     randomdata.RandStringRunes(16),
		OrgAccountBalance: fmt.Sprint(randomdata.Number(1000, 9999)),
		ThirdPartyTransID: randomdata.Alphanumeric(32),
		MSISDN:            fmt.Sprint(txPhoneNumber()),
		FirstName:         randomdata.FirstName(randomdata.Female),
		LastName:          randomdata.SillyName(),
		MiddleName:        randomdata.SillyName(),
	}

	bs, err := json.MarshalIndent(mpesaPayload, "", "\t")
	errs.Panic(err)

	return bytes.NewBuffer(bs)
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

func stkPayloadBuffer() io.Reader {
	stkPayload := payload.STKPayload{
		Body: payload.Body{
			STKCallback: payload.STKCallback{
				MerchantRequestID: randomdata.RandStringRunes(32),
				CheckoutRequestID: randomdata.RandStringRunes(32),
				ResultCode:        0,
				ResultDesc:        "The service request is processed successfully.",
				CallbackMetadata: payload.CallbackMeta{
					Item: []payload.Item{
						{
							Name:  "Amount",
							Value: randomdata.Number(10, 1000),
						},
						{
							Name:  "MpesaReceiptNumber",
							Value: txID(),
						},
						{
							Name:  "TransactionDate",
							Value: txTime(),
						},
						{
							Name:  "PhoneNumber",
							Value: txPhoneNumber(),
						},
					},
				},
			},
		},
	}

	bs, err := json.MarshalIndent(stkPayload, "", "\t")
	errs.Panic(err)

	return bytes.NewBuffer(bs)
}
