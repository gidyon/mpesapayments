package main

import (
	"context"
	"net/http"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/utils/errs"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	b2c_v2 "github.com/gidyon/mpesapayments/pkg/api/b2c/v2"
	c2b "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	stk_v2 "github.com/gidyon/mpesapayments/pkg/api/stk/v2"
	redis "github.com/go-redis/redis/v8"
	"google.golang.org/grpc/grpclog"
	"gorm.io/gorm"
)

// Options contains parameters passed to mpesa gateways
type Options struct {
	SQLDB                 *gorm.DB
	RedisDB               *redis.Client
	Logger                grpclog.LoggerV2
	AuthAPI               auth.API
	MpesaAPI              c2b.LipaNaMPESAServer
	StkAPI                stk.StkPushAPIServer
	StkV2API              stk_v2.StkPushV2Server
	B2CAPI                b2c.B2CAPIServer
	B2CV2API              b2c_v2.B2CV2Server
	DisableMpesaService   bool
	DisableSTKService     bool
	DisableB2CService     bool
	B2CTransactionCharges float32
}

func validateOptions(opt *Options) error {
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
	case opt.AuthAPI == nil:
		err = errs.NilObject("auth API")
	case opt.StkAPI == nil && !opt.DisableSTKService:
		err = errs.NilObject("stk API")
	case opt.MpesaAPI == nil && !opt.DisableMpesaService:
		err = errs.NilObject("mpesa API")
	case opt.B2CAPI == nil && !opt.DisableB2CService:
		err = errs.NilObject("b2c API")
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
