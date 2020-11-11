package mpesapayment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/go-redis/redis"
	"google.golang.org/protobuf/proto"
)

func (mpesaAPI *mpesaAPIServer) updateAccessTokenWorker(ctx context.Context, dur time.Duration) {
	var err error
	for {
		err = mpesaAPI.updateAccessToken()
		if err != nil {
			mpesaAPI.Logger.Errorf("failed to update access token: %v", err)
		} else {
			mpesaAPI.Logger.Infoln("access token updated")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(dur):
		}
	}
}

func (mpesaAPI *mpesaAPIServer) updateAccessToken() error {
	req, err := http.NewRequest(http.MethodGet, mpesaAPI.STKOptions.AccessTokenURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", mpesaAPI.STKOptions.basicToken))

	res, err := mpesaAPI.HTTPClient.Do(req)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("request failed: %v", err)
	}

	resTo := make(map[string]interface{}, 0)
	err = json.NewDecoder(res.Body).Decode(&resTo)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to json decode response: %v", err)
	}

	mpesaAPI.STKOptions.accessToken = fmt.Sprint(resTo["access_token"])

	return nil
}

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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			res, err := mpesaAPI.RedisDB.BRPopLPush(FailedTxList, failedTxListv2, 0).Result()
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
			mpesaTransaction := &mpesapayment.MPESAPayment{}
			err = proto.Unmarshal([]byte(res), mpesaTransaction)
			if err != nil {
				mpesaAPI.Logger.Errorf("failed to proto unmarshal failed mpesa transaction: %v", err)
				continue
			}

			// Validation
			err = ValidateMPESAPayment(mpesaTransaction)
			if err != nil {
				mpesaAPI.Logger.Errorf("validation failed for mpesa transaction: %v", err)
				continue
			}

			mpesaDB, err := GetMpesaDB(mpesaTransaction)
			if err != nil {
				mpesaAPI.Logger.Errorf("failed to get mpesa database model: %v", err)
				continue
			}

			// Save to database
			err = mpesaAPI.SQLDB.Create(mpesaDB).Error
			switch {
			case err == nil:
				count++
				continue
			case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
				mpesaAPI.Logger.Infoln("skipping mpesa transaction since it is available in database")
				continue
			default:
				mpesaAPI.Logger.Errorf("failed to save mpesa transaction: %v", err)
				continue
			}
		}
	}
}
