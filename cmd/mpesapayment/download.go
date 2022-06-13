package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	"github.com/jszwec/csvutil"
	"google.golang.org/grpc/metadata"
)

type dFilter struct {
	PhoneNumbers             []string `json:"phone_numbers,omitempty"`
	StartTimestampUTCSeconds int64    `json:"start_timestamp_utc_seconds,omitempty"`
	EndTimestampUTCSeconds   int64    `json:"end_timestamp_utc_seconds,omitempty"`
}

func getContextFromRequest(r *http.Request, authAPI auth.API) (context.Context, error) {
	var token string
	jwtBearer := r.Header.Get("authorization")
	if jwtBearer == "" {
		token = r.URL.Query().Get("token")
	} else {
		splits := strings.SplitN(jwtBearer, " ", 2)
		if len(splits) < 2 {
			return nil, fmt.Errorf("bad authorization string")

		}
		if !strings.EqualFold(splits[0], expectedScheme) {
			return nil, fmt.Errorf("request unauthenticated with %s", expectedScheme)
		}
		token = splits[1]
	}

	if token == "" {
		return nil, errors.New("missing jwt token in request")
	}

	// Communication context
	ctx := metadata.NewIncomingContext(
		r.Context(), metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token)),
	)

	// Authorize the context
	ctx, err := authAPI.AuthorizeFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to authorize request: %v", err)
	}

	return ctx, nil
}

func downloadB2C(opt *Options) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) (int, error) {
		if r.Method != http.MethodPost {
			return http.StatusBadRequest, errors.New("only POST method is allowed")
		}

		dFilter := &dFilter{}
		err := json.NewDecoder(r.Body).Decode(dFilter)
		if !errors.Is(err, io.EOF) && err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to decode filter in request body: %v", err)
		}

		ctx, err := getContextFromRequest(r, opt.AuthAPI)
		if err != nil {
			return http.StatusBadRequest, err
		}

		_, err = opt.AuthAPI.AuthorizeGroup(ctx, opt.AuthAPI.AdminGroups()...)
		if err != nil {
			return http.StatusBadRequest, err
		}

		var (
			pageToken string
			pageSize  int32 = 1000
			next            = true
		)

		writer := csv.NewWriter(w)

		defer writer.Flush()

		enc := csvutil.NewEncoder(writer)

		for next {
			listRes, err := opt.B2CAPI.ListB2CPayments(ctx, &b2c.ListB2CPaymentsRequest{
				PageToken: pageToken,
				PageSize:  pageSize,
				Filter: &b2c.ListB2CPaymentFilter{
					Msisdns:        dFilter.PhoneNumbers,
					StartTimestamp: dFilter.StartTimestampUTCSeconds,
					EndTimestamp:   dFilter.EndTimestampUTCSeconds,
				},
			})
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("failed to list b2c : %v", err)
			}

			pageToken = listRes.NextPageToken
			if listRes.NextPageToken == "" {
				next = false
			}

			for _, pb := range listRes.B2CPayments {
				err = enc.Encode(pb)
				if err != nil {
					return http.StatusInternalServerError, fmt.Errorf("failed to encode b2c data to csv: %v", err)
				}
			}
		}

		// Set appropriate content type
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=b2c-%s", time.Now().String()[:16]))

		if writer.Error() != nil && !errors.Is(writer.Error(), io.EOF) {
			return http.StatusInternalServerError, fmt.Errorf("failed to create csv: %v", writer.Error())
		}

		return http.StatusOK, nil
	}

	return func(w http.ResponseWriter, r *http.Request) {
		c, err := handler(w, r)
		if err != nil {
			errMsg := fmt.Sprintf("Error while downloading b2c: %v", err)
			opt.Logger.Errorln(errMsg)
			http.Error(w, errMsg, c)
		}
	}
}

func downloadStk(opt *Options) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) (int, error) {
		if r.Method != http.MethodPost {
			return http.StatusBadRequest, errors.New("only POST method is allowed")
		}

		dFilter := &dFilter{}
		err := json.NewDecoder(r.Body).Decode(dFilter)
		if !errors.Is(err, io.EOF) && err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to decode filter in request body: %v", err)
		}

		ctx, err := getContextFromRequest(r, opt.AuthAPI)
		if err != nil {
			return http.StatusBadRequest, err
		}

		_, err = opt.AuthAPI.AuthorizeGroup(ctx, opt.AuthAPI.AdminGroups()...)
		if err != nil {
			return http.StatusBadRequest, err
		}

		var (
			pageToken string
			pageSize  int32 = 1000
			next            = true
		)

		writer := csv.NewWriter(w)

		defer writer.Flush()

		enc := csvutil.NewEncoder(writer)

		for next {
			listRes, err := opt.StkAPI.ListStkTransactions(ctx, &stk.ListStkTransactionsRequest{
				PageToken: pageToken,
				PageSize:  pageSize,
				Filter: &stk.ListStkTransactionFilter{
					Msisdns:        dFilter.PhoneNumbers,
					StartTimestamp: dFilter.StartTimestampUTCSeconds,
					EndTimestamp:   dFilter.EndTimestampUTCSeconds,
				},
			})
			if err != nil {
				return http.StatusInternalServerError, fmt.Errorf("failed to list stks : %v", err)
			}

			pageToken = listRes.NextPageToken
			if listRes.NextPageToken == "" {
				next = false
			}

			for _, pb := range listRes.StkTransactions {
				err = enc.Encode(pb)
				if err != nil {
					return http.StatusInternalServerError, fmt.Errorf("failed to encode stk data to csv: %v", err)
				}
			}
		}

		// Set appropriate content type
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=stk-%s", time.Now().String()[:16]))

		if writer.Error() != nil && !errors.Is(writer.Error(), io.EOF) {
			return http.StatusInternalServerError, fmt.Errorf("failed to create csv: %v", writer.Error())
		}

		return http.StatusOK, nil
	}

	return func(w http.ResponseWriter, r *http.Request) {
		c, err := handler(w, r)
		if err != nil {
			errMsg := fmt.Sprintf("Error while downloading stks: %v", err)
			opt.Logger.Errorln(errMsg)
			http.Error(w, errMsg, c)
		}
	}
}
