package main

import (
	"context"
	"net/http"
	"os"
	"time"

	micro "github.com/gidyon/micro/v2"
	"github.com/gidyon/micro/v2/pkg/config"
	"github.com/gidyon/micro/v2/pkg/healthcheck"
	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/pkg/middleware/grpc/zaplogger"
	"github.com/gidyon/micro/v2/utils/encryption"
	"github.com/gidyon/micro/v2/utils/errs"
	b2capp "github.com/gidyon/mpesapayments/internal/b2c"
	mpesa "github.com/gidyon/mpesapayments/internal/c2b"
	stkapp "github.com/gidyon/mpesapayments/internal/stk"
	"github.com/gidyon/mpesapayments/pkg/api/b2c"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/gorilla/securecookie"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	httpmiddleware "github.com/gidyon/micro/v2/pkg/middleware/http"

	app_grpc_middleware "github.com/gidyon/micro/v2/pkg/middleware/grpc"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

func main() {
	ctx := context.Background()

	apiHashKey, err := encryption.ParseKey([]byte(os.Getenv("API_HASH_KEY")))
	errs.Panic(err)

	apiBlockKey, err := encryption.ParseKey([]byte(os.Getenv("API_BLOCK_KEY")))
	errs.Panic(err)

	// config
	cfg, err := config.New(config.FromFile)
	errs.Panic(err)

	// initialize logger
	errs.Panic(zaplogger.Init(cfg.LogLevel(), ""))

	zaplogger.Log = zaplogger.Log.WithOptions(zap.WithCaller(true))

	appLogger := zaplogger.ZapGrpcLoggerV2(zaplogger.Log)

	app, err := micro.NewService(ctx, cfg, appLogger)
	errs.Panic(err)

	// Recovery middleware
	recoveryUIs, recoverySIs := app_grpc_middleware.AddRecovery()
	app.AddGRPCUnaryServerInterceptors(recoveryUIs...)
	app.AddGRPCStreamServerInterceptors(recoverySIs...)

	// Logging middleware
	logginUIs, loggingSIs := app_grpc_middleware.AddLogging(zaplogger.Log)
	app.AddGRPCUnaryServerInterceptors(logginUIs...)
	app.AddGRPCStreamServerInterceptors(loggingSIs...)

	jwtKey := []byte(os.Getenv("JWT_SIGNING_KEY"))

	// Authentication API
	authAPI, err := auth.NewAPI(&auth.Options{
		SigningKey: jwtKey,
		Issuer:     "MPESA API",
		Audience:   "apis",
	})
	errs.Panic(err)

	// Generate jwt token
	token, err := authAPI.GenToken(context.Background(), &auth.Payload{Group: auth.DefaultAdminGroup()}, time.Now().Add(time.Hour*24))
	if err == nil {
		app.Logger().Infof("Test jwt is %s", token)
	}

	// Authentication middleware
	app.AddGRPCUnaryServerInterceptors(grpc_auth.UnaryServerInterceptor(authAPI.AuthorizeFunc))
	app.AddGRPCStreamServerInterceptors(grpc_auth.StreamServerInterceptor(authAPI.AuthorizeFunc))

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

	app.AddServeMuxOptions(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			EmitUnpopulated: true,
		},
	}))

	app.Start(ctx, func() error {
		// Pagination hasher
		paginationHasher, err := encryption.NewHasher(string(jwtKey))
		errs.Panic(err)

		var (
			disableSTKAPI   = os.Getenv("DISABLE_STK_SERVICE") != ""
			disableMpesaAPI = os.Getenv("DISABLE_MPESA_SERVICE") != ""
			disableB2CAPI   = os.Getenv("DISABLE_B2C_SERVICE") != ""
			mpesaAPI        c2b.LipaNaMPESAServer
			stkAPI          stk.StkPushAPIServer
			b2cAPI          b2c.B2CAPIServer
		)

		if !disableMpesaAPI {
			opt := mpesa.Options{
				PublishChannel:   os.Getenv("PUBLISH_CHANNEL_PAYBILL"),
				RedisKeyPrefix:   os.Getenv("REDIS_KEY_PREFIX"),
				SQLDB:            app.GormDBByName("sqlWrites"),
				RedisDB:          app.RedisClientByName("redisWrites"),
				Logger:           app.Logger(),
				AuthAPI:          authAPI,
				PaginationHasher: paginationHasher,
			}
			// MPESA API
			mpesaAPI, err = mpesa.NewAPIServerMPESA(ctx, &opt)
			errs.Panic(err)

			c2b.RegisterLipaNaMPESAServer(app.GRPCServer(), mpesaAPI)
			errs.Panic(c2b.RegisterLipaNaMPESAHandler(ctx, app.RuntimeMux(), app.ClientConn()))
		}

		if !disableSTKAPI {
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

			opt := stkapp.Options{
				SQLDB:               app.GormDBByName("sqlWrites"),
				RedisDB:             app.RedisClientByName("redisWrites"),
				Logger:              app.Logger(),
				AuthAPI:             authAPI,
				PaginationHasher:    paginationHasher,
				HTTPClient:          http.DefaultClient,
				OptionsSTK:          stkOption,
				PublishChannel:      os.Getenv("PUBLISH_CHANNEL_STK"),
				RedisKeyPrefix:      os.Getenv("REDIS_KEY_PREFIX"),
				DisableMpesaService: disableMpesaAPI,
			}

			// STK Push API
			stkAPI, err = stkapp.NewStkAPI(ctx, &opt, mpesaAPI)
			errs.Panic(err)

			stk.RegisterStkPushAPIServer(app.GRPCServer(), stkAPI)
			errs.Panic(stk.RegisterStkPushAPIHandler(ctx, app.RuntimeMux(), app.ClientConn()))
		}

		if !disableB2CAPI {
			optB2C := &b2capp.OptionsB2C{
				AccessTokenURL:             os.Getenv("MPESA_ACCESS_TOKEN_URL"),
				ConsumerKey:                os.Getenv("SAF_CONSUMER_KEY"),
				ConsumerSecret:             os.Getenv("SAF_CONSUMER_SECRET"),
				QueueTimeOutURL:            os.Getenv("QUEUE_TIMEOUT_URL"),
				ResultURL:                  os.Getenv("RESULT_URL"),
				InitiatorUsername:          os.Getenv("INITIATOR_USERNAME"),
				InitiatorPassword:          os.Getenv("INITIATOR_PASSWORD"),
				InitiatorEncryptedPassword: os.Getenv("INITIATOR_ENCRYPTED_PASSWORD"),
			}

			opt := &b2capp.Options{
				PublishChannel:   os.Getenv("PUBLISH_CHANNEL_B2C"),
				RedisKeyPrefix:   os.Getenv("REDIS_KEY_PREFIX"),
				B2CLocalTopic:    os.Getenv("PUBLISH_CHANNEL_LOCAL"),
				QueryBalanceURL:  os.Getenv("QUERY_BALANCE_URL"),
				B2CURL:           os.Getenv("B2C_URL"),
				ReversalURL:      os.Getenv("REVERSAL_URL"),
				SQLDB:            app.GormDBByName("sqlWrites"),
				RedisDB:          app.RedisClientByName("redisWrites"),
				Logger:           app.Logger(),
				AuthAPI:          authAPI,
				PaginationHasher: paginationHasher,
				HTTPClient:       http.DefaultClient,
				OptionsB2C:       optB2C,
			}
			b2cAPI, err = b2capp.NewB2CAPI(ctx, opt)
			errs.Panic(err)

			b2c.RegisterB2CAPIServer(app.GRPCServer(), b2cAPI)
			b2c.RegisterB2CAPIHandler(ctx, app.RuntimeMux(), app.ClientConn())
		}

		// Options for gateways
		optGateway := &Options{
			SQLDB:               app.GormDBByName("sqlWrites"),
			RedisDB:             app.RedisClientByName("redisWrites"),
			Logger:              app.Logger(),
			AuthAPI:             authAPI,
			MpesaAPI:            mpesaAPI,
			StkAPI:              stkAPI,
			B2CAPI:              b2cAPI,
			DisableMpesaService: disableMpesaAPI,
			DisableSTKService:   disableSTKAPI,
			DisableB2CService:   disableB2CAPI,
			RedisKeyPrefix:      os.Getenv("REDIS_KEY_PREFIX"),
			B2CLocalTopic:       os.Getenv("PUBLISH_CHANNEL_LOCAL"),
		}

		if !disableMpesaAPI {
			// MPESA Paybill confirmation gateway
			paybillGW, err := NewPayBillGateway(ctx, optGateway)
			errs.Panic(err)

			// MPESA Validation gateway
			validationGw, err := NewValidationAPI(ctx, optGateway)
			errs.Panic(err)

			validationPath := firstVal(os.Getenv("VALIDATION_URL_PATH"), "/api/mpestx/validation")
			confirmationPath := firstVal(os.Getenv("CONFIRMATION_URL_PATH"), "/api/mpestx/confirmation")

			app.AddEndpoint(validationPath, validationGw)
			app.AddEndpoint(confirmationPath, paybillGW)

			app.Logger().Infof("Mpesa validation path: %v", validationPath)
			app.Logger().Infof("Mpesa confirmation path: %v", confirmationPath)
		}

		if !disableSTKAPI {
			// MPESA STK Push gateway
			stkGateway, err := NewSTKGateway(ctx, optGateway)
			errs.Panic(err)

			stkCallback := firstVal(os.Getenv("STK_CALLBACK_URL_PATH"), "/api/mpestx/incoming/stkpush")

			app.AddEndpoint(stkCallback, stkGateway)

			app.Logger().Infof("STK callback path: %v", stkCallback)
		}

		if !disableB2CAPI {
			// B2C Mpesa gateway
			b2cGateway, err := NewB2CGateway(ctx, optGateway)
			errs.Panic(err)

			b2cCallback := firstVal(os.Getenv("B2C_CALLBACK_URL_PATH"), "/api/mpestx/incoming/b2c")

			app.AddEndpoint(b2cCallback, b2cGateway)

			app.Logger().Infof("B2C callback path: %v", b2cCallback)
		}

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
