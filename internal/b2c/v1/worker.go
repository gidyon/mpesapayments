package b2c

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gidyon/mpesapayments/pkg/utils/httputils"
)

func (b2cAPI *b2cAPIServer) updateAccessTokenWorker(ctx context.Context, dur time.Duration) {
	var err error
	for {
		err = b2cAPI.updateAccessToken()
		if err != nil {
			b2cAPI.Logger.Errorf("failed to update access token: %v", err)
			time.Sleep(10 * time.Second)
			continue
		} else {
			b2cAPI.Logger.Infoln("access token updated")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(dur):
		}
	}
}

func (b2cAPI *b2cAPIServer) updateAccessToken() error {
	req, err := http.NewRequest(http.MethodGet, b2cAPI.OptionsB2C.AccessTokenURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", b2cAPI.OptionsB2C.basicToken))

	httputils.DumpRequest(req, "B2C ACCESS TOKEN REQUEST")

	res, err := b2cAPI.HTTPClient.Do(req)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("request failed: %v", err)
	}

	httputils.DumpResponse(res, "B2C ACCESS TOKEN RESPONSE")

	resTo := make(map[string]interface{})
	err = json.NewDecoder(res.Body).Decode(&resTo)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to json decode response: %v", err)
	}

	b2cAPI.OptionsB2C.accessToken = fmt.Sprint(resTo["access_token"])

	return nil
}

func (b2cAPI *b2cAPIServer) subscriptionsWorker(ctx context.Context) {
	ch := b2cAPI.RedisDB.Subscribe(ctx, AddPrefix(b2cAPI.Options.B2CLocalTopic, b2cAPI.RedisKeyPrefix)).Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			// We want to get the request id and unsubscribe the goroutine
			requestID := msg.Payload
			b2cAPI.unsubcribe(requestID)
		}
	}
}
