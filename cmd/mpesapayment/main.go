package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gidyon/micro"
	"github.com/gidyon/micro/pkg/config"
	"github.com/gidyon/micro/pkg/grpc/auth"
	"github.com/gidyon/micro/pkg/healthcheck"
	"github.com/gidyon/micro/utils/encryption"
	"github.com/gidyon/micro/utils/errs"
	mpesa "github.com/gidyon/mpesapayments/internal/mpesapayment"
	stkapp "github.com/gidyon/mpesapayments/internal/stk"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/gorilla/securecookie"

	httpmiddleware "github.com/gidyon/micro/pkg/http"

	app_grpc_middleware "github.com/gidyon/micro/pkg/grpc/middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
)

func main() {
	ctx := context.Background()

	apiHashKey, err := encryption.ParseKey([]byte(os.Getenv("API_HASH_KEY")))
	errs.Panic(err)

	apiBlockKey, err := encryption.ParseKey([]byte(os.Getenv("API_BLOCK_KEY")))
	errs.Panic(err)

	cfg, err := config.New(config.FromFile)
	errs.Panic(err)

	app, err := micro.NewService(ctx, cfg, micro.NewLogger(cfg.ServiceName()))
	errs.Panic(err)

	// Recovery middleware
	recoveryUIs, recoverySIs := app_grpc_middleware.AddRecovery()
	app.AddGRPCUnaryServerInterceptors(recoveryUIs...)
	app.AddGRPCStreamServerInterceptors(recoverySIs...)

	jwtKey := []byte(os.Getenv("JWT_SIGNING_KEY"))

	// Authentication API
	authAPI, err := auth.NewAPI(jwtKey, "USSD Log API", "users")
	errs.Panic(err)

	// Generate jwt token
	token, err := authAPI.GenToken(context.Background(), &auth.Payload{Group: auth.AdminGroup()}, time.Now().Add(time.Hour*24))
	if err == nil {
		app.Logger().Infof("Test jwt is %s", token)
	}

	app.AddGRPCUnaryServerInterceptors(grpc_auth.UnaryServerInterceptor(authAPI.AuthFunc))
	app.AddGRPCStreamServerInterceptors(grpc_auth.StreamServerInterceptor(authAPI.AuthFunc))

	// Readiness health check
	app.AddEndpoint("/api/mpestx/health/ready", healthcheck.RegisterProbe(&healthcheck.ProbeOptions{
		Service: app,
		Type:    healthcheck.ProbeReadiness,
	}))

	// Liveness health check
	app.AddEndpoint("/api/mpestx/health/live", healthcheck.RegisterProbe(&healthcheck.ProbeOptions{
		Service: app,
		Type:    healthcheck.ProbeLiveNess,
	}))

	sc := securecookie.New(apiHashKey, apiBlockKey)

	// Cookie based authentication
	app.AddHTTPMiddlewares(httpmiddleware.CookieToJWTMiddleware(&httpmiddleware.CookieJWTOptions{
		SecureCookie: sc,
		AuthHeader:   auth.Header(),
		AuthScheme:   auth.Scheme(),
		CookieName:   auth.JWTCookie(),
	}))

	app.AddHTTPMiddlewares(httpmiddleware.SupportCORS)

	app.Start(ctx, func() error {
		// Pagination hasher
		paginationHasher, err := encryption.NewHasher(string(jwtKey))
		errs.Panic(err)

		stkOption := &stkapp.OptionsSTK{
			AccessTokenURL:    os.Getenv("MPESA_ACCESS_TOKEN_URL"),
			ConsumerKey:       os.Getenv("SAF_CONSUMER_KEY"),
			ConsumerSecret:    os.Getenv("SAF_CONSUMER_SECRET"),
			BusinessShortCode: os.Getenv("BUSINESS_SHORT_CODE"),
			AccountReference:  os.Getenv("MPESA_ACCOUNT_REFERENCE"),
			Timestamp:         os.Getenv("MPESA_ACCESS_TIMESTAMP"),
			PassKey:           os.Getenv("LNM_PASSKEY"),
			CallBackURL:       os.Getenv("MPESA_CALLBACK_URL"),
			PostURL:           os.Getenv("MPESA_POST_URL"),
			QueryURL:          os.Getenv("MPESA_QUERY_URL"),
		}

		disablePub := os.Getenv("DISABLE_REDIS") != ""
		disableMpesaAPI := os.Getenv("DISABLE_MPESA_SERVICE") != ""

		opt := stkapp.Options{
			SQLDB:               app.GormDBByName("sqlWrites"),
			RedisDB:             app.RedisClientByName("redisWrites"),
			Logger:              app.Logger(),
			AuthAPI:             authAPI,
			PaginationHasher:    paginationHasher,
			HTTPClient:          http.DefaultClient,
			OptionsSTK:          stkOption,
			PublishChannelSTK:   os.Getenv("PUBLISH_CHANNEL_STK"),
			PublishChannelMpesa: os.Getenv("PUBLISH_CHANNEL_PAYBILL"),
			DisableMpesaService: disableMpesaAPI,
			DisablePublishing:   disablePub,
		}

		// MPESA API
		mpesaAPI, err := mpesa.NewAPIServerMPESA(ctx, &opt)
		errs.Panic(err)

		mpesapayment.RegisterLipaNaMPESAServer(app.GRPCServer(), mpesaAPI)
		errs.Panic(mpesapayment.RegisterLipaNaMPESAHandler(ctx, app.RuntimeMux(), app.ClientConn()))

		// STK Push API
		stkAPI, err := stkapp.NewStkAPI(ctx, &opt, mpesaAPI)
		errs.Panic(err)

		stk.RegisterStkPushAPIServer(app.GRPCServer(), stkAPI)
		errs.Panic(stk.RegisterStkPushAPIHandler(ctx, app.RuntimeMux(), app.ClientConn()))

		// Options for gateways
		optGateway := &Options{
			SQLDB:               app.GormDBByName("sqlWrites"),
			RedisDB:             app.RedisClientByName("redisWrites"),
			Logger:              app.Logger(),
			AuthAPI:             authAPI,
			MpesaAPI:            mpesaAPI,
			StkAPI:              stkAPI,
			DisablePublishing:   disablePub,
			DisableMpesaService: disableMpesaAPI,
		}

		// MPESA Paybill confirmation gateway
		paybillGW, err := NewPayBillGateway(ctx, optGateway)
		errs.Panic(err)

		// MPESA Validation gateway
		validationGw, err := NewValidationAPI(ctx, optGateway)
		errs.Panic(err)

		// MPESA STK Push gateway
		stkGateway, err := NewSTKGateway(ctx, optGateway)
		errs.Panic(err)

		validationPath := firstVal(os.Getenv("VALIDATION_URL_PATH"), "/api/mpestx/validation")
		confirmationPath := firstVal(os.Getenv("CONFIRMATION_URL_PATH"), "/api/mpestx/confirmation")
		stkCallback := firstVal(os.Getenv("STK_CALLBACK_URL_PATH"), "/api/mpestx/incoming/stkpush")

		// Register gateways
		app.AddEndpoint(validationPath, validationGw)
		app.AddEndpoint(confirmationPath, paybillGW)
		app.AddEndpoint(stkCallback, stkGateway)

		app.Logger().Infof("Mpesa validation path: %v", validationPath)
		app.Logger().Infof("Mpesa confirmation path: %v", confirmationPath)
		app.Logger().Infof("STK callback path: %v", stkCallback)

		return nil
	})
}

func firstVal(vals ...string) string {
	for _, val := range vals {
		if val != "" {
			return val
		}
	}
	return ""
}
