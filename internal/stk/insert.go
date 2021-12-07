package stk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/stk"
	redis "github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
)

func valFunc(v1, v2 string) string {
	if v1 == "" || v1 == "0" {
		return v2
	}
	return v1
}

func (stkAPI *stkAPIServer) insertWorker(ctx context.Context) {
	ticker := time.NewTicker(stkAPI.insertTimeOut)
	defer ticker.Stop()

	incomingPayments := make([]*incomingPayment, 0, bulkInsertSize)

	createFn := func() []*PayloadStk {
		txs := make([]*PayloadStk, 0, len(incomingPayments))
		for _, incomingPayment := range incomingPayments {
			txs = append(txs, incomingPayment.payment)
		}
		return txs
	}

	callback := func(publish bool) {

		for _, v := range incomingPayments {

			// By value because the slice will be reset
			go func(incomingPayment incomingPayment) {
				stkPayload := incomingPayment.payment

				// Get payload key
				key := GetMpesaSTKPushKey(stkPayload.PhoneNumber, stkAPI.RedisKeyPrefix)

				// Delete key to allow other STK transactions to proceed
				defer func() {
					stkAPI.RedisDB.Del(ctx, key)
				}()

				publish := false

				// Get STK initiator id
				initiatorID, err := stkAPI.RedisDB.Get(ctx, key).Result()
				switch {
				case err == nil:
				case errors.Is(err, redis.Nil):
					return
				default:
					stkAPI.Logger.Errorf("failed to get initiator key from cache: %v", err)
					return
				}

				// Initiator payload key
				initiatorKey := GetMpesaSTKPayloadKey(initiatorID, stkAPI.RedisKeyPrefix)

				// Get initiator payload
				val, err := stkAPI.RedisDB.Get(ctx, initiatorKey).Result()
				switch {
				case err == nil:
					publish = true
				case errors.Is(err, redis.Nil):
					stkAPI.Logger.Warningln("no value set in key (initiator payload)")
					publish = false
				default:
					stkAPI.Logger.Errorf("failed to get initiator payload from cache: %v", err)
					publish = false
				}

				if publish && stkPayload.Succeeded && incomingPayment.publish {
					payloadID := valFunc(fmt.Sprint(stkPayload.PayloadID), stkPayload.TransactionID)
					stkAPI.Logger.Infof("publishing stk %s to consumers", payloadID)

					// Get stk push initiator payload
					payload := &stk.InitiateSTKPushRequest{}
					err = proto.Unmarshal([]byte(val), payload)
					if err != nil {
						stkAPI.Logger.Errorf("failed unmarshal stk push payload: %v", err)
						return
					}

					// Publish the stk payment to consumers
					_, err = stkAPI.PublishStkPayload(
						stkAPI.ctxAdmin, &stk.PublishStkPayloadRequest{
							PayloadId: payloadID,
							Payload:   payload.Payload,
						})
					if err != nil {
						stkAPI.Logger.Errorf("failed to publish stk: %v", err)
						return
					}
				}
			}(*v)
		}

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
					callback(true)
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					stkAPI.Logger.Infoln("insert of duplicate stk payloads skipped (from ticker)")
					callback(false)
				default:
					stkAPI.Logger.Errorf("failed to save stk paylods (from ticker): %v", err)
					callback(false)
				}
			}

		case stkPayload := <-stkAPI.insertChan:
			incomingPayments = append(incomingPayments, stkPayload)
			if len(incomingPayments) >= (chanSize - 1) {
				err := stkAPI.SQLDB.CreateInBatches(createFn(), bulkInsertSize).Error
				switch {
				case err == nil:
					stkAPI.Logger.Infof("bulk inserted %d stk payloads (from channel)", len(incomingPayments))
					callback(true)
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					stkAPI.Logger.Infoln("insert of duplicate stk payloads skipped (from channel)")
					callback(false)
				default:
					stkAPI.Logger.Errorf("failed to save stk paylods (from channel): %v", err)
					callback(false)
				}
			}
		}
	}
}
