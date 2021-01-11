package b2c

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

func (b2cAPI *b2cAPIServer) updateAccessTokenWorker(ctx context.Context, dur time.Duration) {
	var err error
	for {
		err = b2cAPI.updateAccessToken()
		if err != nil {
			b2cAPI.Logger.Errorf("failed to update access token: %v", err)
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

	res, err := b2cAPI.HTTPClient.Do(req)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("request failed: %v", err)
	}

	resTo := make(map[string]interface{}, 0)
	err = json.NewDecoder(res.Body).Decode(&resTo)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to json decode response: %v", err)
	}

	b2cAPI.OptionsB2C.accessToken = fmt.Sprint(resTo["access_token"])

	return nil
}

func (b2cAPI *b2cAPIServer) subscriptionsWorker(ctx context.Context) {
	ch := b2cAPI.RedisDB.Subscribe(ctx, b2cAPI.B2CLocalTopic).Channel()

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
