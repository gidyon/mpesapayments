package stk

import (
	"context"
	"strings"
	"time"

	stk_model "github.com/gidyon/mpesapayments/internal/stk"
)

func (stkAPI *stkAPIServer) insertWorker(ctx context.Context) {
	ticker := time.NewTicker(stkAPI.insertTimeOut)
	defer ticker.Stop()

	incomingPayments := make([]*incomingPayment, 0, bulkInsertSize)

	createFn := func() []*stk_model.STKTransaction {
		txs := make([]*stk_model.STKTransaction, 0, len(incomingPayments))
		for _, incomingPayment := range incomingPayments {
			txs = append(txs, incomingPayment.payment)
		}
		return txs
	}

	callback := func() {
		// Reset stuffs
		ticker.Reset(stkAPI.insertTimeOut)
		incomingPayments = incomingPayments[0:0]
	}

	chanSize := cap(stkAPI.insertChan)

	if chanSize <= 2 {
		stkAPI.insertChan = make(chan *incomingPayment, 100)
		chanSize = cap(stkAPI.insertChan)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(incomingPayments) > 0 {
				err := stkAPI.SQLDB.CreateInBatches(createFn(), bulkInsertSize).Error
				switch {
				case err == nil:
					stkAPI.Logger.Infof("bulk inserted %d stk payloads (from ticker)", len(incomingPayments))
					callback()
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					stkAPI.Logger.Infoln("insert of duplicate stk payloads skipped (from ticker)")
					callback()
				default:
					stkAPI.Logger.Errorf("failed to save stk paylods (from ticker): %v", err)
					callback()
				}
			}

		case stkPayload := <-stkAPI.insertChan:
			incomingPayments = append(incomingPayments, stkPayload)
			if len(incomingPayments) >= (chanSize - 1) {
				err := stkAPI.SQLDB.CreateInBatches(createFn(), bulkInsertSize).Error
				switch {
				case err == nil:
					stkAPI.Logger.Infof("bulk inserted %d stk payloads (from channel)", len(incomingPayments))
					callback()
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					stkAPI.Logger.Infoln("insert of duplicate stk payloads skipped (from channel)")
					callback()
				default:
					stkAPI.Logger.Errorf("failed to save stk paylods (from channel): %v", err)
					callback()
				}
			}
		}
	}
}
