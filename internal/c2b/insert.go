package c2b

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/c2b"
)

func valFunc(v1, v2 string) string {
	if v1 == "" || v1 == "0" {
		return v2
	}
	return v1
}

func (c2bAPI *c2bAPIServer) insertWorker(ctx context.Context) {
	ticker := time.NewTicker(c2bAPI.insertTimeOut)
	defer ticker.Stop()

	incomingPayments := make([]*incomingPayment, 0, bulkInsertSize)

	createFn := func() []*PaymentMpesa {
		txs := make([]*PaymentMpesa, 0, len(incomingPayments))
		for _, incomingPayment := range incomingPayments {
			txs = append(txs, incomingPayment.payment)
		}
		return txs
	}

	updateFn := func() {
		for _, incomingPayment := range incomingPayments {
			if incomingPayment.publish {
				// By value because the slice will be reset
				go func(paymentDB PaymentMpesa) {
					paymentID := valFunc(fmt.Sprint(paymentDB.PaymentID), paymentDB.TransactionID)
					// Publish the transaction
					_, err := c2bAPI.PublishC2BPayment(
						c2bAPI.ctxAdmin, &c2b.PublishC2BPaymentRequest{
							PaymentId:   paymentID,
							InitiatorId: paymentDB.MSISDN,
						})
					if err != nil {
						c2bAPI.Logger.Errorf("failed to publish lnm payment with id: %%v", err)
						return
					}
				}(*incomingPayment.payment)
			}
		}

		ticker.Reset(c2bAPI.insertTimeOut)
		incomingPayments = incomingPayments[0:0]
	}

	chanSize := cap(c2bAPI.insertChan)

	if chanSize <= 2 {
		c2bAPI.insertChan = make(chan *incomingPayment, 100)
		chanSize = cap(c2bAPI.insertChan)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(incomingPayments) > 0 {
				err := c2bAPI.SQLDB.CreateInBatches(createFn(), bulkInsertSize).Error
				switch {
				case err == nil:
					c2bAPI.Logger.Infof("bulk inserted %d mpesa payments (from ticker)", len(incomingPayments))
					updateFn()
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					c2bAPI.Logger.Infoln("insert of duplicate mpesa payments skipped (from ticker)")
					updateFn()
				default:
					c2bAPI.Logger.Errorf("failed to save stk paylods (from ticker): %v", err)
				}
			}

		case paymentDB := <-c2bAPI.insertChan:
			incomingPayments = append(incomingPayments, paymentDB)
			if len(incomingPayments) >= (chanSize - 2) {
				err := c2bAPI.SQLDB.CreateInBatches(createFn(), bulkInsertSize).Error
				switch {
				case err == nil:
					c2bAPI.Logger.Infof("bulk inserted %d mpesa payments (from channel)", len(incomingPayments))
					updateFn()
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					c2bAPI.Logger.Infoln("insert of duplicate mpesa payments skipped (from channel)")
					updateFn()
				default:
					c2bAPI.Logger.Errorf("failed to save stk paylods (from channel): %v", err)
				}
			}
		}
	}
}
