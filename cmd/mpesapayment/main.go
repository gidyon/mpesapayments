package main

import (
	"context"
	"net/http"
	"os"

	"github.com/gidyon/micro"
	"github.com/gidyon/micro/pkg/config"
	"github.com/gidyon/micro/utils/healthcheck"
	mpesa "github.com/gidyon/mpesapayments/internal/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/services/pkg/auth"
	"github.com/gidyon/services/pkg/utils/encryption"
	"github.com/gidyon/services/pkg/utils/errs"
	"github.com/gorilla/securecookie"

	httpmiddleware "github.com/gidyon/micro/pkg/http"
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

	// Readiness health check
	app.AddEndpoint("/api/mpestx/health/ready", healthcheck.RegisterProbe(&healthcheck.ProbeOptions{
		Service:      app,
		Type:         healthcheck.ProbeReadiness,
		AutoMigrator: func() error { return nil },
	}))

	// Liveness health check
	app.AddEndpoint("/api/mpestx/health/live", healthcheck.RegisterProbe(&healthcheck.ProbeOptions{
		Service:      app,
		Type:         healthcheck.ProbeLiveNess,
		AutoMigrator: func() error { return nil },
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

		// MPESA API
		mpesaAPI, err := mpesa.NewAPIServerMPESA(ctx, &mpesa.Options{
			SQLDB:         app.GormDBByName("sqlWrites"),
			RedisDB:       app.RedisClientByName("redisWrites"),
			Logger:        app.Logger(),
			JWTSigningKey: []byte(os.Getenv("JWT_SIGNING_KEY")),
			HTTPClient:    http.DefaultClient,
			STKOptions: &mpesa.STKOptions{
				AccessTokenURL:    os.Getenv("MPESA_ACCESS_TOKEN_URL"),
				ConsumerKey:       os.Getenv("SAF_CONSUMER_KEY"),
				ConsumerSecret:    os.Getenv("SAF_CONSUMER_SECRET"),
				BusinessShortCode: os.Getenv("BUSINESS_SHORT_CODE"),
				AccountReference:  os.Getenv("MPESA_ACCOUNT_REFERENCE"),
				Timestamp:         os.Getenv("MPESA_ACCESS_TIMESTAMP"),
				Password:          os.Getenv("MPESA_ACCESS_PASSWORD"),
				CallBackURL:       os.Getenv("MPESA_CALLBACK_URL"),
				PostURL:           os.Getenv("MPESA_POST_URL"),
				QueryURL:          os.Getenv("MPESA_QUERY_URL"),
			},
		})
		errs.Panic(err)

		mpesapayment.RegisterLipaNaMPESAServer(app.GRPCServer(), mpesaAPI)
		errs.Panic(mpesapayment.RegisterLipaNaMPESAHandler(ctx, app.RuntimeMux(), app.ClientConn()))

		// MPESA Paybill gateway
		paybillGW, err := NewPayBillGateway(ctx, &Options{
			SQLDB:         app.GormDBByName("sqlWrites"),
			RedisDB:       app.RedisClientByName("redisWrites"),
			Logger:        app.Logger(),
			JWTSigningKey: []byte(os.Getenv("JWT_SIGNING_KEY")),
			MpesaAPI:      mpesaAPI,
		})
		errs.Panic(err)

		// MPESA Validation gateway
		validationGw, err := NewValidationAPI(ctx, &Options{
			SQLDB:         app.GormDBByName("sqlWrites"),
			RedisDB:       app.RedisClientByName("redisWrites"),
			Logger:        app.Logger(),
			JWTSigningKey: []byte(os.Getenv("JWT_SIGNING_KEY")),
			MpesaAPI:      mpesaAPI,
		})
		errs.Panic(err)

		// MPESA STK Push gateway
		stkGateway, err := NewSTKGateway(ctx, &Options{
			SQLDB:         app.GormDBByName("sqlWrites"),
			RedisDB:       app.RedisClientByName("redisWrites"),
			Logger:        app.Logger(),
			JWTSigningKey: []byte(os.Getenv("JWT_SIGNING_KEY")),
			MpesaAPI:      mpesaAPI,
		})
		errs.Panic(err)

		app.AddEndpoint("/api/mpestx/validation", validationGw)
		app.AddEndpoint("/api/mpestx/confirmation", paybillGW)
		app.AddEndpoint("/api/mpestx/incoming/stkpush", stkGateway)

		return nil
	})
}
