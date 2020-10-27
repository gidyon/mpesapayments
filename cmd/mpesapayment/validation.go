package main

import (
	"context"
	"net/http"

	"github.com/gidyon/services/pkg/utils/errs"
)

type validationAPI struct {
	*Options
}

func validateOptions(opt *Options) error {
	// Validate
	var err error
	switch {
	case opt == nil:
		err = errs.NilObject("options")
	case opt.SQLDB == nil:
		err = errs.NilObject("sqlDB")
	case opt.RedisDB == nil:
		err = errs.NilObject("redisDB")
	case opt.Logger == nil:
		err = errs.NilObject("logger")
	case opt.JWTSigningKey == nil:
		err = errs.NilObject("jwt key")
	case opt.MpesaAPI == nil:
		err = errs.NilObject("mpesa server")
	}
	return err
}

// NewValidationAPI creates a validation API for paybill
func NewValidationAPI(ctx context.Context, opt *Options) (http.Handler, error) {
	// Validate
	var err error
	switch {
	case ctx == nil:
		err = errs.NilObject("context")
	default:
		err = validateOptions(opt)
	}
	if err != nil {
		return nil, err
	}
	return &validationAPI{Options: opt}, nil
}

func (vapi *validationAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vapi.Logger.Infoln("received incoming validation request")
	vapi.Logger.Infof("request url is: %s%v", r.Host, r.URL.RequestURI())
	vapi.Logger.Infof("method is: %v", r.Method)
}
