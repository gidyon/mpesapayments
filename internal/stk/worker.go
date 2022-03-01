package stk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
	redis "github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
)

func (stkAPI *stkAPIServer) updateAccessTokenWorker(ctx context.Context, dur time.Duration) {
	var err error
	for {
		err = stkAPI.updateAccessToken()
		if err != nil {
			stkAPI.Logger.Errorf("failed to update access token: %v", err)
		} else {
			stkAPI.Logger.Infoln("access token updated")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(dur):
		}
	}
}

func (stkAPI *stkAPIServer) updateAccessToken() error {
	req, err := http.NewRequest(http.MethodGet, stkAPI.OptionsSTK.AccessTokenURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", stkAPI.OptionsSTK.basicToken))

	httputils.DumpRequest(req, "STK ACCESS TOKEN REQUEST")

	res, err := stkAPI.HTTPClient.Do(req)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("request failed: %v", err)
	}

	httputils.DumpResponse(res, "STK ACCESS TOKEN RESPONSE")

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status code OK got: %v", res.Status)
	}

	resTo := make(map[string]interface{})
	err = json.NewDecoder(res.Body).Decode(&resTo)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to json decode response: %v", err)
	}

	stkAPI.OptionsSTK.accessToken = fmt.Sprint(resTo["access_token"])

	return nil
}

func (stkAPI *stkAPIServer) worker(ctx context.Context, dur time.Duration) {
	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := stkAPI.saveFailedStks()
			switch {
			case err == nil || errors.Is(err, io.EOF):
			case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			default:
				stkAPI.Logger.Errorf("error while running worker: %v", err)
			}
		}
	}
}

func (stkAPI *stkAPIServer) saveFailedStks() error {

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	count := 0

	defer func() {
		if count > 0 {
			stkAPI.Logger.Infof("%d failed transactions saved in database", count)
		}
	}()

loop:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			res, err := stkAPI.RedisDB.BRPopLPush(
				ctx, FailedTxList, failedTxListv2, 5*time.Minute,
			).Result()
			switch {
			case err == nil:
			case errors.Is(err, redis.Nil):
			default:
				if err != nil {
					return err
				}
				if res == "" {
					goto loop
				}
			}

			// Unmarshal
			stkPayload := &stk.StkPayload{}
			err = proto.Unmarshal([]byte(res), stkPayload)
			if err != nil {
				stkAPI.Logger.Errorf("failed to proto unmarshal failed mpesa transaction: %v", err)
				goto loop
			}

			// Validation
			err = ValidateStkPayload(stkPayload)
			if err != nil {
				stkAPI.Logger.Errorf("validation failed for stk transaction: %v", err)
				goto loop
			}

			stkPayloadDB, err := StkPayloadDB(stkPayload)
			if err != nil {
				stkAPI.Logger.Errorf("failed to get mpesa database model: %v", err)
				goto loop
			}

			// Save to database
			err = stkAPI.SQLDB.Create(stkPayloadDB).Error
			switch {
			case err == nil:
				count++
			case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
				stkAPI.Logger.Infoln("skipping mpesa transaction since it is available in database")
				goto loop
			default:
				stkAPI.Logger.Errorf("failed to save mpesa transaction: %v", err)
				goto loop
			}

			stkAPI.Logger.Infoln("stk payload saved in database")
		}
	}
}
