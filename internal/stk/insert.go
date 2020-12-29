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

const maxCap = 10000

func valFunc(v1, v2 string) string {
	if v1 == "" || v1 == "0" {
		return v2
	}
	return v1
}

func (stkAPI *stkAPIServer) insertWorker(ctx context.Context) {
	ticker := time.NewTicker(stkAPI.insertTimeOut)
	defer ticker.Stop()

	stkPayloads := make([]*PayloadStk, 0, bulkInsertSize)

	callback := func(publish bool) {

		if !stkAPI.DisablePublishing && publish {

			// Publish to consumers
			stkAPI.Logger.Infof("publishing %d stk result to consumers", len(stkPayloads))

			for _, stkPayload := range stkPayloads {

				// By value because the slice will be reset
				go func(stkPayload PayloadStk) {
					// Get payload key
					key := GetMpesaSTKPushKey(stkPayload.PhoneNumber)

					// Delete key to allow other STK transactions to proceed
					defer func() {
						stkAPI.RedisDB.Del(ctx, key)
					}()

					publish := false

					// Get STK initiator key
					initiatorKey, err := stkAPI.RedisDB.Get(ctx, key).Result()
					switch {
					case err == nil:
						publish = true
					case errors.Is(err, redis.Nil):
						stkAPI.Logger.Warningln("no value set in key (initiator key)")
						return
					default:
						stkAPI.Logger.Errorf("failed to get initiator key from cache: %v", err)
						return
					}

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

					if publish && stkPayload.Succeeded {
						payloadID := valFunc(fmt.Sprint(stkPayload.PayloadID), stkPayload.MpesaReceiptNumber)
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
				}(*stkPayload)
			}
		}

		// Reset stuffs
		ticker.Reset(stkAPI.insertTimeOut)
		stkPayloads = stkPayloads[0:0]
	}

	chanSize := cap(stkAPI.insertChan)

	if chanSize <= 2 {
		stkAPI.insertChan = make(chan *PayloadStk, 100)
		chanSize = cap(stkAPI.insertChan)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if len(stkPayloads) > 0 {
				err := stkAPI.SQLDB.CreateInBatches(stkPayloads, bulkInsertSize).Error
				switch {
				case err == nil:
					stkAPI.Logger.Infof("bulk inserted %d stk payloads (from ticker)", len(stkPayloads))
					callback(true)
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					stkAPI.Logger.Infoln("insert of duplicate stk payloads skipped (from ticker)")
					callback(false)
				default:
					stkAPI.Logger.Errorf("failed to save stk paylods (from ticker): %v", err)
				}
			}

		case stkPayload := <-stkAPI.insertChan:
			stkPayloads = append(stkPayloads, stkPayload)
			if len(stkPayloads) >= (chanSize - 1) {
				err := stkAPI.SQLDB.CreateInBatches(stkPayloads, bulkInsertSize).Error
				switch {
				case err == nil:
					stkAPI.Logger.Infof("bulk inserted %d stk payloads (from channel)", len(stkPayloads))
					callback(true)
				case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
					stkAPI.Logger.Infoln("insert of duplicate stk payloads skipped (from channel)")
					callback(false)
				default:
					stkAPI.Logger.Errorf("failed to save stk paylods (from channel): %v", err)
				}
			}
		}
	}
}
