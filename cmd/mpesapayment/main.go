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
	c2bapp_v1 "github.com/gidyon/mpesapayments/internal/c2b/v1"
	stkapp_v1 "github.com/gidyon/mpesapayments/internal/stk/v1"
	b2c_v1 "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	c2b_v1 "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	stk_v1 "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	app_grpc_middleware "github.com/gidyon/micro/v2/pkg/middleware/grpc"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

func main() {
	ctx := context.Background()

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
	token, err := authAPI.GenToken(context.Background(), &auth.Payload{Names: "TEST", ID: "0"}, time.Now().Add(time.Hour*24))
	if err == nil {
		app.Logger().Infof("test jwt is [%s]", token)
	}

	// Default jwt
	app.AddEndpointFunc("/api/mpestx/jwt/default", func(w http.ResponseWriter, r *http.Request) {
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
			disableSTKAPI = os.Getenv("DISABLE_STK_SERVICE") != ""
			disableC2BAPI = os.Getenv("DISABLE_MPESA_SERVICE") != ""
			disableB2CAPI = os.Getenv("DISABLE_B2C_SERVICE") != ""
			mpesaAPI      c2b_v1.LipaNaMPESAServer
			stkAPI        stk_v1.StkPushAPIServer
			b2cAPI        b2c_v1.B2CAPIServer
		)

		if !disableC2BAPI {
			opt := c2bapp_v1.Options{
				PublishChannel:   os.Getenv("C2B_PUBLISH_CHANNEL"),
				RedisKeyPrefix:   os.Getenv("REDIS_KEY_PREFIX"),
				SQLDB:            app.GormDBByName("sqlWrites"),
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
			stkOption := &stkapp_v1.OptionsSTK{
				AccessTokenURL:    os.Getenv("MPESA_ACCESS_TOKEN_URL"),
				ConsumerKey:       firstVal(os.Getenv("STK_CONSUMER_KEY"), os.Getenv("SAF_CONSUMER_KEY")),
				ConsumerSecret:    firstVal(os.Getenv("STK_CONSUMER_SECRET"), os.Getenv("SAF_CONSUMER_SECRET")),
				BusinessShortCode: os.Getenv("STK_BUSINESS_SHORT_CODE"),
				AccountReference:  os.Getenv("STK_MPESA_ACCOUNT_REFERENCE"),
				Timestamp:         os.Getenv("STK_MPESA_ACCESS_TIMESTAMP"),
				PassKey:           os.Getenv("STK_LNM_PASSKEY"),
				CallBackURL:       os.Getenv("STK_MPESA_CALLBACK_URL"),
				PostURL:           os.Getenv("STK_MPESA_POST_URL"),
				QueryURL:          os.Getenv("STK_MPESA_QUERY_URL"),
			}

			opt := stkapp_v1.Options{
				SQLDB:               app.GormDBByName("sqlWrites"),
				RedisDB:             app.RedisClientByName("redisWrites"),
				Logger:              app.Logger(),
				AuthAPI:             authAPI,
				HTTPClient:          http.DefaultClient,
				OptionsSTK:          stkOption,
				PublishChannel:      os.Getenv("STK_PUBLISH_CHANNEL"),
				DisableMpesaService: disableC2BAPI,
			}

			stkAPI, err = stkapp_v1.NewStkAPI(ctx, &opt, mpesaAPI)
			errs.Panic(err)

			stk_v1.RegisterStkPushAPIServer(app.GRPCServer(), stkAPI)
			errs.Panic(stk_v1.RegisterStkPushAPIHandler(ctx, app.RuntimeMux(), app.ClientConn()))
		}

		if !disableB2CAPI {
			optB2C := &b2capp_v1.OptionsB2C{
				AccessTokenURL:             os.Getenv("MPESA_ACCESS_TOKEN_URL"),
				ConsumerKey:                firstVal(os.Getenv("B2C_CONSUMER_KEY"), os.Getenv("SAF_CONSUMER_KEY")),
				ConsumerSecret:             firstVal(os.Getenv("B2C_CONSUMER_SECRET"), os.Getenv("SAF_CONSUMER_SECRET")),
				QueueTimeOutURL:            os.Getenv("B2C_QUEUE_TIMEOUT_URL"),
				ResultURL:                  os.Getenv("B2C_RESULT_URL"),
				InitiatorUsername:          os.Getenv("B2C_INITIATOR_USERNAME"),
				InitiatorPassword:          os.Getenv("B2C_INITIATOR_PASSWORD"),
				InitiatorEncryptedPassword: os.Getenv("B2C_INITIATOR_ENCRYPTED_PASSWORD"),
			}

			opt := &b2capp_v1.Options{
				PublishChannel:   os.Getenv("B2C_PUBLISH_CHANNEL"),
				RedisKeyPrefix:   os.Getenv("REDIS_KEY_PREFIX"),
				B2CLocalTopic:    os.Getenv("B2C_PUBLISH_CHANNEL_LOCAL"),
				QueryBalanceURL:  os.Getenv("B2C_QUERY_BALANCE_URL"),
				B2CURL:           os.Getenv("B2C_URL"),
				ReversalURL:      os.Getenv("B2C_REVERSAL_URL"),
				SQLDB:            app.GormDBByName("sqlWrites"),
				RedisDB:          app.RedisClientByName("redisWrites"),
				Logger:           app.Logger(),
				AuthAPI:          authAPI,
				PaginationHasher: paginationHasher,
				HTTPClient:       http.DefaultClient,
				OptionsB2C:       optB2C,
			}
			b2cAPI, err = b2capp_v1.NewB2CAPI(ctx, opt)
			errs.Panic(err)

			b2c_v1.RegisterB2CAPIServer(app.GRPCServer(), b2cAPI)
			errs.Panic(b2c_v1.RegisterB2CAPIHandler(ctx, app.RuntimeMux(), app.ClientConn()))
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
			B2CAPI:                b2cAPI,
			DisableMpesaService:   disableC2BAPI,
			DisableSTKService:     disableSTKAPI,
			DisableB2CService:     disableB2CAPI,
			RedisKeyPrefix:        os.Getenv("REDIS_KEY_PREFIX"),
			B2CLocalTopic:         os.Getenv("B2C_PUBLISH_CHANNEL_LOCAL"),
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

			stkCallback := firstVal(os.Getenv("STK_CALLBACK_URL_PATH"), "/api/mpestx/stkpush/incoming")

			app.AddEndpoint(stkCallback, stkGateway)

			app.Logger().Infof("STK callback path: %v", stkCallback)
		}

		if !disableB2CAPI {
			// B2C Mpesa gateway
			b2cGateway, err := NewB2CGateway(ctx, optGateway)
			errs.Panic(err)

			b2cCallback := firstVal(os.Getenv("B2C_CALLBACK_URL_PATH"), "/api/mpestx/b2c/incoming")

			app.AddEndpoint(b2cCallback, b2cGateway)

			app.Logger().Infof("B2C callback path: %v", b2cCallback)
		}

		// Endpoint for uploading blast file
		app.AddEndpointFunc("/api/mpestx/uploads/blast", uploadHandler(optGateway))

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
