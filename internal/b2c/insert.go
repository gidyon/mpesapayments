package b2c

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/b2c"
)

const maxCap = 10000

func valFunc(v1, v2 string) string {
	if v1 == "" || v1 == "0" {
		return v2
	}
	return v1
}

func (b2cAPI *b2cAPIServer) insertWorker(ctx context.Context) {
	ticker := time.NewTicker(b2cAPI.insertTimeOut)
	defer ticker.Stop()

	incomingPayments := make([]*incomingPayment, 0, bulkInsertSize)

	transactionsFn := func() []*Payment {
		txs := make([]*Payment, 0, len(incomingPayments))
		for _, incomingPayment := range incomingPayments {
			txs = append(txs, incomingPayment.payment)
		}
		return txs
	}

	callback := func() {
		for _, v := range incomingPayments {
			if v.publish {
				// By value because the slice will be reset
				go func(incomingPayment incomingPayment) {
					paymentID := valFunc(fmt.Sprint(incomingPayment.payment.PaymentID), incomingPayment.payment.TransactionID)
					// Publish the transaction
					_, err := b2cAPI.PublishB2CPayment(
						b2cAPI.ctxAdmin, &b2c.PublishB2CPaymentRequest{
							PaymentId:   paymentID,
							InitiatorId: incomingPayment.payment.InitiatorID,
						})
					if err != nil {
						b2cAPI.Logger.Errorf("failed to publish b2c trnasaction with id: %%v", err)
						return
					}
				}(*v)
			}
		}
		ticker.Reset(b2cAPI.insertTimeOut)
		incomingPayments = incomingPayments[0:0]
	}

	chanSize := cap(b2cAPI.insertChan)

	if chanSize <= 2 {
		b2cAPI.insertChan = make(chan *incomingPayment, 100)
		chanSize = cap(b2cAPI.insertChan)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(incomingPayments) > 0 {
				err := b2cAPI.SQLDB.CreateInBatches(transactionsFn(), bulkInsertSize).Error
				switch {
				case err == nil:
					b2cAPI.Logger.Infof("bulk inserted %d b2c transactions (from ticker)", len(incomingPayments))
					callback()
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					b2cAPI.Logger.Infoln("insert of duplicate b2c transactions skipped (from ticker)")
					callback()
				default:
					b2cAPI.Logger.Errorf("failed to save b2c transactions (from ticker): %v", err)
				}
			}

		case incomingPayment := <-b2cAPI.insertChan:
			incomingPayments = append(incomingPayments, incomingPayment)
			if len(incomingPayments) >= (chanSize - 1) {
				err := b2cAPI.SQLDB.CreateInBatches(transactionsFn(), bulkInsertSize).Error
				switch {
				case err == nil:
					b2cAPI.Logger.Infof("bulk inserted %d b2c transactions (from channel)", len(incomingPayments))
					callback()
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					b2cAPI.Logger.Infoln("insert of duplicate b2c transactions skipped (from channel)")
					callback()
				default:
					b2cAPI.Logger.Errorf("failed to save b2c transactions (from channel): %v", err)
				}
			}
		}
	}
}
