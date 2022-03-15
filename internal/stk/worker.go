package stk

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
