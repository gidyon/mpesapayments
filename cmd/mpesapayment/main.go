package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	micro "github.com/gidyon/micro/v2"
	"github.com/gidyon/micro/v2/pkg/config"
	"github.com/gidyon/micro/v2/pkg/healthcheck"
	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	"github.com/gidyon/micro/v2/pkg/middleware/grpc/zaplogger"
	"github.com/gidyon/micro/v2/utils/encryption"
	"github.com/gidyon/micro/v2/utils/errs"
	b2capp_v1 "github.com/gidyon/mpesapayments/internal/b2c/v1"
	b2capp_v2 "github.com/gidyon/mpesapayments/internal/b2c/v2"
	c2bapp_v1 "github.com/gidyon/mpesapayments/internal/c2b/v1"
	stkapp_v1 "github.com/gidyon/mpesapayments/internal/stk/v1"
	stkapp_v2 "github.com/gidyon/mpesapayments/internal/stk/v2"
	b2c_v1 "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	b2c_v2 "github.com/gidyon/mpesapayments/pkg/api/b2c/v2"
	c2b_v1 "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	stk_v1 "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	stk_v2 "github.com/gidyon/mpesapayments/pkg/api/stk/v2"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	app_grpc_middleware "github.com/gidyon/micro/v2/pkg/middleware/grpc"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

func main() {
	ctx := context.Background()

	// config
	cfg, err := config.New()
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
	token, err := authAPI.GenToken(context.Background(), &auth.Payload{Names: "TEST", ID: "0"}, time.Now().Add(time.Hour*24))
	if err == nil {
		app.Logger().Infof("test jwt is [%s]", token)
	}

	// Default jwt
	app.AddEndpointFunc("/api/mpestx/jwt/default", func(w http.ResponseWriter, _ *http.Request) {
		token, err := authAPI.GenToken(ctx, &auth.Payload{Names: "TEST", ID: "0"}, time.Now().Add(time.Hour*24*365))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("content-type", "application/json")

		err = json.NewEncoder(w).Encode(map[string]string{"default_token": token})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

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

	// app.AddHTTPMiddlewares(httpmiddleware.SupportCORS)

	app.AddRuntimeMuxOptions(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			EmitUnpopulated: true,
		},
	}))

	app.Start(ctx, func() error {
		// Pagination hasher
		paginationHasher, err := encryption.NewHasher(string(jwtKey))
		errs.Panic(err)

		var (
			disableSTKAPI = os.Getenv("DISABLE_STK_SERVICE") != ""
			disableC2BAPI = os.Getenv("DISABLE_MPESA_SERVICE") != ""
			disableB2CAPI = os.Getenv("DISABLE_B2C_SERVICE") != ""
			mpesaAPI      c2b_v1.LipaNaMPESAServer
			stkAPI        stk_v1.StkPushAPIServer
			stkCallback1  = os.Getenv("STK_RESULT_URL")
			stkAPIV2      stk_v2.StkPushV2Server
			stkCallback2  = os.Getenv("STK_RESULT_URL_V2")
			b2cAPI        b2c_v1.B2CAPIServer
			b2cCallback1  = os.Getenv("B2C_RESULT_URL")
			b2cAPIV2      b2c_v2.B2CV2Server
			b2cCallbackV2 = os.Getenv("B2C_RESULT_URL_V2")
		)

		if !disableC2BAPI {
			opt := c2bapp_v1.Options{
				PublishChannel:   os.Getenv("C2B_PUBLISH_CHANNEL"),
				RedisKeyPrefix:   os.Getenv("REDIS_KEY_PREFIX"),
				SQLDB:            app.GormDBByName("sqlWrites").Debug(),
				RedisDB:          app.RedisClientByName("redisWrites"),
				Logger:           app.Logger(),
				AuthAPI:          authAPI,
				PaginationHasher: paginationHasher,
			}
			// MPESA API
			mpesaAPI, err = c2bapp_v1.NewAPIServerMPESA(ctx, &opt)
			errs.Panic(err)

			c2b_v1.RegisterLipaNaMPESAServer(app.GRPCServer(), mpesaAPI)
			errs.Panic(c2b_v1.RegisterLipaNaMPESAHandler(ctx, app.RuntimeMux(), app.ClientConn()))
		}

		if !disableSTKAPI {
			// V1
			stkAPI, err = stkapp_v1.NewStkAPI(ctx, &stkapp_v1.Options{
				SQLDB:   app.GormDBByName("sqlWrites").Debug(),
				RedisDB: app.RedisClientByName("redisWrites"),
				Logger:  app.Logger(),
				AuthAPI: authAPI,
				OptionsSTK: &stkapp_v1.OptionsSTK{
					AccessTokenURL:    os.Getenv("MPESA_ACCESS_TOKEN_URL"),
					ConsumerKey:       firstVal(os.Getenv("STK_CONSUMER_KEY"), os.Getenv("SAF_CONSUMER_KEY")),
					ConsumerSecret:    firstVal(os.Getenv("STK_CONSUMER_SECRET"), os.Getenv("SAF_CONSUMER_SECRET")),
					BusinessShortCode: os.Getenv("STK_BUSINESS_SHORT_CODE"),
					AccountReference:  os.Getenv("STK_MPESA_ACCOUNT_REFERENCE"),
					Timestamp:         os.Getenv("STK_MPESA_ACCESS_TIMESTAMP"),
					PassKey:           os.Getenv("STK_LNM_PASSKEY"),
					CallBackURL:       stkCallback1,
					PostURL:           os.Getenv("STK_MPESA_POST_URL"),
					QueryURL:          os.Getenv("STK_MPESA_QUERY_URL"),
				},
				HTTPClient:                http.DefaultClient,
				UpdateAccessTokenDuration: 0,
				WorkerDuration:            0,
				InitiatorExpireDuration:   0,
				PublishChannel:            os.Getenv("STK_PUBLISH_CHANNEL"),
				DisableMpesaService:       disableC2BAPI,
			}, mpesaAPI)
			errs.Panic(err)

			stk_v1.RegisterStkPushAPIServer(app.GRPCServer(), stkAPI)
			errs.Panic(stk_v1.RegisterStkPushAPIHandler(ctx, app.RuntimeMux(), app.ClientConn()))

			// V2
			stkAPIV2, err = stkapp_v2.NewStkAPI(ctx, &stkapp_v2.Options{
				SQLDB:   app.GormDBByName("sqlWrites").Debug(),
				RedisDB: app.RedisClientByName("redisWrites"),
				Logger:  app.Logger(),
				AuthAPI: authAPI,
				OptionSTK: &stkapp_v2.OptionSTK{
					AccessTokenURL:    os.Getenv("MPESA_ACCESS_TOKEN_URL"),
					ConsumerKey:       firstVal(os.Getenv("STK_CONSUMER_KEY"), os.Getenv("SAF_CONSUMER_KEY")),
					ConsumerSecret:    firstVal(os.Getenv("STK_CONSUMER_SECRET"), os.Getenv("SAF_CONSUMER_SECRET")),
					BusinessShortCode: os.Getenv("STK_BUSINESS_SHORT_CODE"),
					AccountReference:  os.Getenv("STK_MPESA_ACCOUNT_REFERENCE"),
					Timestamp:         os.Getenv("STK_MPESA_ACCESS_TIMESTAMP"),
					PassKey:           os.Getenv("STK_LNM_PASSKEY"),
					CallBackURL:       stkCallback2,
					PostURL:           os.Getenv("STK_MPESA_POST_URL"),
					QueryURL:          os.Getenv("STK_MPESA_QUERY_URL"),
				},
				HTTPClient:                http.DefaultClient,
				UpdateAccessTokenDuration: 0,
			})
			errs.Panic(err)

			stk_v2.RegisterStkPushV2Server(app.GRPCServer(), stkAPIV2)
			errs.Panic(stk_v2.RegisterStkPushV2Handler(ctx, app.RuntimeMux(), app.ClientConn()))
		}

		if !disableB2CAPI {
			// V1
			b2cAPI, err = b2capp_v1.NewB2CAPI(ctx, &b2capp_v1.Options{
				PublishChannel:  os.Getenv("B2C_PUBLISH_CHANNEL"),
				QueryBalanceURL: os.Getenv("B2C_QUERY_BALANCE_URL"),
				B2CURL:          os.Getenv("B2C_URL"),
				ReversalURL:     os.Getenv("B2C_REVERSAL_URL"),
				SQLDB:           app.GormDBByName("sqlWrites").Debug(),
				RedisDB:         app.RedisClientByName("redisWrites"),
				Logger:          app.Logger(),
				AuthAPI:         authAPI,
				HTTPClient:      http.DefaultClient,
				OptionB2C: &b2capp_v1.OptionB2C{
					ConsumerKey:                firstVal(os.Getenv("B2C_CONSUMER_KEY"), os.Getenv("SAF_CONSUMER_KEY")),
					ConsumerSecret:             firstVal(os.Getenv("B2C_CONSUMER_SECRET"), os.Getenv("SAF_CONSUMER_SECRET")),
					AccessTokenURL:             os.Getenv("MPESA_ACCESS_TOKEN_URL"),
					QueueTimeOutURL:            os.Getenv("B2C_QUEUE_TIMEOUT_URL"),
					ResultURL:                  b2cCallback1,
					InitiatorUsername:          os.Getenv("B2C_INITIATOR_USERNAME"),
					InitiatorPassword:          os.Getenv("B2C_INITIATOR_PASSWORD"),
					InitiatorEncryptedPassword: os.Getenv("B2C_INITIATOR_ENCRYPTED_PASSWORD"),
					PublicKeyCertificateFile:   "",
				},
				TransactionCharges: 0,
			})
			errs.Panic(err)

			b2c_v1.RegisterB2CAPIServer(app.GRPCServer(), b2cAPI)
			errs.Panic(b2c_v1.RegisterB2CAPIHandler(ctx, app.RuntimeMux(), app.ClientConn()))

			// V2
			b2cAPIV2, err = b2capp_v2.NewB2CAPI(ctx, &b2capp_v2.Options{
				QueryBalanceURL: os.Getenv("B2C_QUERY_BALANCE_URL"),
				B2CURL:          os.Getenv("B2C_URL"),
				ReversalURL:     os.Getenv("B2C_REVERSAL_URL"),
				SQLDB:           app.GormDBByName("sqlWrites").Debug(),
				RedisDB:         app.RedisClientByName("redisWrites"),
				Logger:          app.Logger(),
				AuthAPI:         authAPI,
				HTTPClient:      http.DefaultClient,
				OptionB2C: &b2capp_v2.OptionB2C{
					ConsumerKey:                firstVal(os.Getenv("B2C_CONSUMER_KEY"), os.Getenv("SAF_CONSUMER_KEY")),
					ConsumerSecret:             firstVal(os.Getenv("B2C_CONSUMER_SECRET"), os.Getenv("SAF_CONSUMER_SECRET")),
					AccessTokenURL:             os.Getenv("MPESA_ACCESS_TOKEN_URL"),
					QueueTimeOutURL:            os.Getenv("B2C_QUEUE_TIMEOUT_URL"),
					ResultURL:                  b2cCallbackV2,
					InitiatorUsername:          os.Getenv("B2C_INITIATOR_USERNAME"),
					InitiatorPassword:          os.Getenv("B2C_INITIATOR_PASSWORD"),
					InitiatorEncryptedPassword: os.Getenv("B2C_INITIATOR_ENCRYPTED_PASSWORD"),
					PublicKeyCertificateFile:   "",
				},
				TransactionCharges: 0,
			})
			errs.Panic(err)

			b2c_v2.RegisterB2CV2Server(app.GRPCServer(), b2cAPIV2)
			errs.Panic(b2c_v2.RegisterB2CV2Handler(ctx, app.RuntimeMux(), app.ClientConn()))
		}

		b2cTxCost, _ := strconv.ParseFloat(os.Getenv("B2C_TRANSACTION_CHARGES"), 32)

		// Options for gateways
		optGateway := &Options{
			SQLDB:                 app.GormDBByName("sqlWrites"),
			RedisDB:               app.RedisClientByName("redisWrites"),
			Logger:                app.Logger(),
			AuthAPI:               authAPI,
			MpesaAPI:              mpesaAPI,
			StkAPI:                stkAPI,
			StkV2API:              stkAPIV2,
			B2CAPI:                b2cAPI,
			B2CV2API:              b2cAPIV2,
			DisableMpesaService:   disableC2BAPI,
			DisableSTKService:     disableSTKAPI,
			DisableB2CService:     disableB2CAPI,
			B2CTransactionCharges: float32(b2cTxCost),
		}

		if !disableC2BAPI {
			// MPESA Paybill confirmation gateway
			paybillGW, err := NewPayBillGateway(ctx, optGateway)
			errs.Panic(err)

			// MPESA Validation gateway
			validationGw, err := NewValidationAPI(ctx, optGateway)
			errs.Panic(err)

			validationPath := firstVal(os.Getenv("VALIDATION_URL_PATH"), "/api/mpestx/c2b/incoming/validation")
			confirmationPath := firstVal(os.Getenv("CONFIRMATION_URL_PATH"), "/api/mpestx/c2b/incoming/confirmation")

			app.AddEndpoint(validationPath, validationGw)
			app.AddEndpoint(confirmationPath, paybillGW)

			// Legacy endpoints
			// /api/mpestx/confirmation/
			app.AddEndpoint("/api/mpestx/confirmation/", paybillGW)

			// /api/mpestx/validation/
			app.AddEndpoint("/api/mpestx/validation/", validationGw)

			// /api/mpestx/confirmation/
			app.AddEndpoint("/api/mpestx/confirmation", paybillGW)

			// /api/mpestx/validation/
			app.AddEndpoint("/api/mpestx/validation", validationGw)

			app.Logger().Infof("Mpesa validation path: %v", validationPath)
			app.Logger().Infof("Mpesa confirmation path: %v", confirmationPath)
		}

		if !disableSTKAPI {
			// MPESA STK Push gateway
			stkGateway, err := NewSTKGateway(ctx, optGateway)
			errs.Panic(err)

			// V1 endpoint
			app.AddEndpointFunc("/api/mpestx/stkpush/incoming/v1", stkGateway.ServeStk)
			app.Logger().Infof("STK callback path: %v", stkCallback1)

			// V2 endpoint
			app.AddEndpointFunc("/api/mpestx/stkpush/incoming/v2", stkGateway.ServeStkV2)
			app.Logger().Infof("STK V2 callback path: %v", stkCallback2)
		}

		if !disableB2CAPI {
			// B2C Mpesa gateway
			b2cGateway, err := NewB2CGateway(ctx, optGateway)
			errs.Panic(err)

			// V1 endpoint
			app.AddEndpointFunc("/api/mpestx/b2c/incoming/v1", b2cGateway.ServeHTTP)
			app.Logger().Infof("B2C callback path: %v", b2cCallback1)

			// V2 Endpoint
			app.AddEndpointFunc("/api/mpestx/b2c/incoming/v2", b2cGateway.ServeHttpV2)
			app.Logger().Infof("B2C V2 callback path: %v", b2cCallbackV2)
		}

		// Endpoint for uploading blast file
		app.AddEndpointFunc("/api/mpestx/uploads/blast", uploadHandler(optGateway))

		// Download API
		app.AddEndpointFunc("/api/mpestx/dowloads/stk", downloadStk(optGateway))
		app.AddEndpointFunc("/api/mpestx/dowloads/b2c", downloadB2C(optGateway))

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
