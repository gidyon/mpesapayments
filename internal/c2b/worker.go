package c2b

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	redis "github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
)

func (mpesaAPI *mpesaAPIServer) worker(ctx context.Context, dur time.Duration) {
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := mpesaAPI.saveFailedTransactions()
			switch {
			case err == nil || errors.Is(err, io.EOF):
			case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			default:
				mpesaAPI.Logger.Errorf("error while running worker: %v", err)
			}
		}
	}
}

func (mpesaAPI *mpesaAPIServer) saveFailedTransactions() error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	count := 0

	defer func() {
		mpesaAPI.Logger.Infof("%d failed transactions saved in database", count)
	}()

loop:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			res, err := mpesaAPI.RedisDB.BRPopLPush(
				ctx,
				mpesaAPI.AddPrefix(FailedTxList),
				mpesaAPI.AddPrefix(failedTxListv2),
				5*time.Minute,
			).Result()
			switch {
			case err == nil:
			case errors.Is(err, redis.Nil):
			default:
				if err != nil {
					return err
				}
				if res == "" {
					return nil
				}
			}

			// Unmarshal
			mpesaTransaction := &c2b.C2BPayment{}
			err = proto.Unmarshal([]byte(res), mpesaTransaction)
			if err != nil {
				mpesaAPI.Logger.Errorf("failed to proto unmarshal failed mpesa transaction: %v", err)
				goto loop
			}

			// Validation
			err = ValidateC2BPayment(mpesaTransaction)
			if err != nil {
				mpesaAPI.Logger.Errorf("validation failed for mpesa transaction: %v", err)
				goto loop
			}

			mpesaDB, err := GetMpesaDB(mpesaTransaction)
			if err != nil {
				mpesaAPI.Logger.Errorf("failed to get mpesa database model: %v", err)
				goto loop
			}

			// Save to database
			err = mpesaAPI.SQLDB.Create(mpesaDB).Error
			switch {
			case err == nil:
				count++
			case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
				mpesaAPI.Logger.Infoln("skipping mpesa transaction since it is available in database")
				goto loop
			default:
				mpesaAPI.Logger.Errorf("failed to save mpesa transaction: %v", err)
				goto loop
			}

			mpesaAPI.Logger.Infoln("mpesa transaction saved in database")
		}
	}
}
