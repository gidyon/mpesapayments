package mpesapayment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
)

func valFunc(v1, v2 string) string {
	if v1 == "" || v1 == "0" {
		return v2
	}
	return v1
}

func (mpesaAPI *mpesaAPIServer) insertWorker(ctx context.Context) {
	ticker := time.NewTicker(mpesaAPI.insertTimeOut)
	defer ticker.Stop()

	paymentsDB := make([]*PaymentMpesa, 0, bulkInsertSize)

	updateFn := func() {

		if !mpesaAPI.DisablePublishing {

			// Publish to consumers
			mpesaAPI.Logger.Infof("publishing %d mpesa payments to consumers", len(paymentsDB))

			for _, paymentDB := range paymentsDB {

				// By value because the slice will be reset
				go func(paymentDB PaymentMpesa) {
					if !mpesaAPI.DisablePublishing {
						paymentID := valFunc(fmt.Sprint(paymentDB.PaymentID), paymentDB.TxID)

						// Publish the transaction
						_, err := mpesaAPI.PublishMpesaPayment(
							mpesaAPI.ctxAdmin, &mpesapayment.PublishMpesaPaymentRequest{
								PaymentId:   paymentID,
								InitiatorId: paymentDB.MSISDN,
							})
						if err != nil {
							mpesaAPI.Logger.Errorf("failed to publish lnm payment with id: %%v", err)
							return
						}
					}
				}(*paymentDB)
			}
		}

		ticker.Reset(mpesaAPI.insertTimeOut)
		paymentsDB = paymentsDB[0:0]
	}

	chanSize := cap(mpesaAPI.insertChan)

	if chanSize <= 2 {
		mpesaAPI.insertChan = make(chan *PaymentMpesa, 100)
		chanSize = cap(mpesaAPI.insertChan)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(paymentsDB) > 0 {
				err := mpesaAPI.SQLDB.CreateInBatches(paymentsDB, bulkInsertSize).Error
				switch {
				case err == nil:
					mpesaAPI.Logger.Infof("bulk inserted %d mpesa payments (from ticker)", len(paymentsDB))
					updateFn()
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					mpesaAPI.Logger.Infoln("insert of duplicate mpesa payments skipped (from ticker)")
					updateFn()
				default:
					mpesaAPI.Logger.Errorf("failed to save stk paylods (from ticker): %v", err)
				}
			}

		case paymentDB := <-mpesaAPI.insertChan:
			paymentsDB = append(paymentsDB, paymentDB)
			if len(paymentsDB) >= (chanSize - 2) {
				err := mpesaAPI.SQLDB.CreateInBatches(paymentsDB, bulkInsertSize).Error
				switch {
				case err == nil:
					mpesaAPI.Logger.Infof("bulk inserted %d mpesa payments (from channel)", len(paymentsDB))
					updateFn()
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					mpesaAPI.Logger.Infoln("insert of duplicate mpesa payments skipped (from channel)")
					updateFn()
				default:
					mpesaAPI.Logger.Errorf("failed to save stk paylods (from channel): %v", err)
				}
			}
		}
	}
}
