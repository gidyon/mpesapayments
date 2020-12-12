package main

import (
	"context"
	"net/http"

	"github.com/gidyon/micro/pkg/grpc/auth"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/gidyon/services/pkg/utils/errs"
	redis "github.com/go-redis/redis/v8"
	"google.golang.org/grpc/grpclog"
	"gorm.io/gorm"
)

// Options contains parameters passed to NewUSSDGateway
type Options struct {
	SQLDB               *gorm.DB
	RedisDB             *redis.Client
	Logger              grpclog.LoggerV2
	AuthAPI             auth.API
	MpesaAPI            mpesapayment.LipaNaMPESAServer
	StkAPI              stk.StkPushAPIServer
	DisableMpesaService bool
	DisablePublishing   bool
}

func validateOptions(opt *Options) error {
	var err error
	switch {
	case opt == nil:
		err = errs.NilObject("options")
	case opt.SQLDB == nil:
		err = errs.NilObject("sqlDB")
	case opt.RedisDB == nil && !opt.DisablePublishing:
		err = errs.NilObject("redisDB")
	case opt.Logger == nil:
		err = errs.NilObject("logger")
	case opt.AuthAPI == nil:
		err = errs.NilObject("auth API")
	case opt.StkAPI == nil:
		err = errs.NilObject("stk API")
	case opt.MpesaAPI == nil:
		err = errs.NilObject("mpesa API")
	}
	return err
}

type validationAPI struct {
	*Options
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
