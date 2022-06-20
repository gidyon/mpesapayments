package b2c

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/micro/v2"
	"github.com/gidyon/micro/v2/pkg/conn"
	"github.com/gidyon/micro/v2/pkg/mocks/mocks"
	b2c_model "github.com/gidyon/mpesapayments/internal/b2c"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	redis "github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func TestB2CPayment(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "B2CPayment Suite")
}

var (
	B2CAPIServer *b2cAPIServer
	B2CAPI       b2c.B2CAPIServer
	workerChan   chan struct{}
	ctx          context.Context
)

const (
	dbAddress = "localhost:3306"
	schema    = "mpesapayments"
)

func startDB() (*gorm.DB, error) {
	return conn.OpenGormConn(&conn.DBOptions{
		Dialect:  "mysql",
		Address:  dbAddress,
		User:     "root",
		Password: "hakty11",
		Schema:   schema,
	})
}

type client int

func (client) Do(*http.Request) (*http.Response, error) {
	return &http.Response{Body: httptest.NewRecorder().Result().Body}, nil
}

var _ = BeforeSuite(func() {
	workerChan = make(chan struct{})
	ctx = context.Background()

	rand.Seed(time.Now().UnixNano())

	// Start real databases
	db, err := startDB()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(db.AutoMigrate(&b2c_model.Payment{})).ShouldNot(HaveOccurred())

	redisDB := conn.OpenRedisConn(&redis.Options{
		Addr: "localhost:6379",
	})

	logger := micro.NewLogger("B2CPayment App", zerolog.TraceLevel)

	authAPI := &mocks.AuthAPIMock{}

	Expect(err).ShouldNot(HaveOccurred())

	optB2C := &OptionB2C{
		ConsumerKey:                randomdata.RandStringRunes(32),
		ConsumerSecret:             randomdata.RandStringRunes(32),
		AccessTokenURL:             "https://sandbox.safaricom.co.ke/oauth/v1/generate",
		QueueTimeOutURL:            "https://sandbox.safaricom.co.ke/oauth/v1/generate",
		ResultURL:                  "https://sandbox.safaricom.co.ke/oauth/v1/generate",
		InitiatorUsername:          randomdata.SillyName(),
		InitiatorPassword:          randomdata.Alphanumeric(32),
		InitiatorEncryptedPassword: randomdata.RandStringRunes(96),
		PublicKeyCertificateFile:   "",
	}

	opt := &Options{
		PublishChannel:  "test",
		QueryBalanceURL: "https://sandbox.safaricom.co.ke/mpesa/accountbalance/v1/query",
		B2CURL:          "https://sandbox.safaricom.co.ke/mpesa/b2c/v1/paymentrequest",
		ReversalURL:     "https://sandbox.safaricom.co.ke/mpesa/reversal/v1/request",
		SQLDB:           db,
		RedisDB:         redisDB,
		Logger:          logger,
		AuthAPI:         authAPI,
		HTTPClient:      client(1),
		OptionB2C:       optB2C,
	}

	// Create MPESA payments API
	B2CAPI, err = NewB2CAPI(ctx, opt)
	Expect(err).ShouldNot(HaveOccurred())

	var ok bool
	B2CAPIServer, ok = B2CAPI.(*b2cAPIServer)
	Expect(ok).Should(BeTrue())

	_, err = NewB2CAPI(ctx, nil)
	Expect(err).Should(HaveOccurred())

	// Validate options for btc
	opt.OptionB2C = nil
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.OptionB2C = optB2C
	opt.OptionB2C.AccessTokenURL = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.OptionB2C.AccessTokenURL = "https://sandbox.safaricom.co.ke/oauth/v1/generate"
	opt.OptionB2C.ConsumerKey = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.OptionB2C.ConsumerKey = randomdata.RandStringRunes(32)
	opt.OptionB2C.ConsumerSecret = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.OptionB2C.ConsumerSecret = randomdata.RandStringRunes(32)
	opt.OptionB2C.InitiatorEncryptedPassword = ""
	opt.OptionB2C.InitiatorPassword = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.OptionB2C.InitiatorEncryptedPassword = randomdata.RandStringRunes(96)
	opt.OptionB2C.InitiatorPassword = randomdata.RandStringRunes(32)
	opt.OptionB2C.PublicKeyCertificateFile = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).ShouldNot(HaveOccurred())

	opt.OptionB2C.QueueTimeOutURL = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.OptionB2C.QueueTimeOutURL = "https://sandbox.safaricom.co.ke/oauth/v1/generate"
	opt.OptionB2C.ResultURL = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.OptionB2C.ResultURL = "https://sandbox.safaricom.co.ke/oauth/v1/generate"

	// Validate options for service
	opt.PublishChannel = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.PublishChannel = "test"
	opt.QueryBalanceURL = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.QueryBalanceURL = "https://sandbox.safaricom.co.ke/mpesa/accountbalance/v1/query"
	opt.B2CURL = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.B2CURL = "https://sandbox.safaricom.co.ke/mpesa/accountbalance/v1/query"
	opt.ReversalURL = ""
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.ReversalURL = "https://sandbox.safaricom.co.ke/mpesa/accountbalance/v1/query"
	opt.RedisDB = nil
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.RedisDB = redisDB
	opt.SQLDB = nil
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.SQLDB = db
	opt.Logger = nil
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.Logger = logger
	opt.AuthAPI = nil
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.AuthAPI = authAPI
	opt.HTTPClient = nil
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.HTTPClient = client(1)
	opt.OptionB2C = nil
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).Should(HaveOccurred())

	opt.OptionB2C = optB2C
	_, err = NewB2CAPI(ctx, opt)
	Expect(err).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	// <-workerChan
	time.Sleep(10 * time.Second)
})

// Declarations for Ginkgo DSL
type Done ginkgo.Done
type Benchmarker ginkgo.Benchmarker

var GinkgoWriter = ginkgo.GinkgoWriter
var GinkgoRandomSeed = ginkgo.GinkgoRandomSeed
var GinkgoParallelNode = ginkgo.GinkgoParallelNode
var GinkgoT = ginkgo.GinkgoT
var CurrentGinkgoTestDescription = ginkgo.CurrentGinkgoTestDescription
var RunSpecs = ginkgo.RunSpecs
var RunSpecsWithDefaultAndCustomReporters = ginkgo.RunSpecsWithDefaultAndCustomReporters
var RunSpecsWithCustomReporters = ginkgo.RunSpecsWithCustomReporters
var Skip = ginkgo.Skip
var Fail = ginkgo.Fail
var GinkgoRecover = ginkgo.GinkgoRecover
var Describe = ginkgo.Describe
var FDescribe = ginkgo.FDescribe
var PDescribe = ginkgo.PDescribe
var XDescribe = ginkgo.XDescribe
var Context = ginkgo.Context
var FContext = ginkgo.FContext
var PContext = ginkgo.PContext
var XContext = ginkgo.XContext
var When = ginkgo.When
var FWhen = ginkgo.FWhen
var PWhen = ginkgo.PWhen
var XWhen = ginkgo.XWhen
var It = ginkgo.It
var FIt = ginkgo.FIt
var PIt = ginkgo.PIt
var XIt = ginkgo.XIt
var Specify = ginkgo.Specify
var FSpecify = ginkgo.FSpecify
var PSpecify = ginkgo.PSpecify
var XSpecify = ginkgo.XSpecify
var By = ginkgo.By
var Measure = ginkgo.Measure
var FMeasure = ginkgo.FMeasure
var PMeasure = ginkgo.PMeasure
var XMeasure = ginkgo.XMeasure
var BeforeSuite = ginkgo.BeforeSuite
var AfterSuite = ginkgo.AfterSuite
var SynchronizedBeforeSuite = ginkgo.SynchronizedBeforeSuite
var SynchronizedAfterSuite = ginkgo.SynchronizedAfterSuite
var BeforeEach = ginkgo.BeforeEach
var JustBeforeEach = ginkgo.JustBeforeEach
var JustAfterEach = ginkgo.JustAfterEach
var AfterEach = ginkgo.AfterEach

// Declarations for Gomega DSL
var RegisterFailHandler = gomega.RegisterFailHandler
var RegisterFailHandlerWithT = gomega.RegisterFailHandlerWithT
var RegisterTestingT = gomega.RegisterTestingT
var InterceptGomegaFailures = gomega.InterceptGomegaFailures
var Ω = gomega.Ω
var Expect = gomega.Expect
var ExpectWithOffset = gomega.ExpectWithOffset
var Eventually = gomega.Eventually
var EventuallyWithOffset = gomega.EventuallyWithOffset
var Consistently = gomega.Consistently
var ConsistentlyWithOffset = gomega.ConsistentlyWithOffset
var SetDefaultEventuallyTimeout = gomega.SetDefaultEventuallyTimeout
var SetDefaultEventuallyPollingInterval = gomega.SetDefaultEventuallyPollingInterval
var SetDefaultConsistentlyDuration = gomega.SetDefaultConsistentlyDuration
var SetDefaultConsistentlyPollingInterval = gomega.SetDefaultConsistentlyPollingInterval
var NewWithT = gomega.NewWithT
var NewGomegaWithT = gomega.NewGomegaWithT

// Declarations for Gomega Matchers
var Equal = gomega.Equal
var BeEquivalentTo = gomega.BeEquivalentTo
var BeIdenticalTo = gomega.BeIdenticalTo
var BeNil = gomega.BeNil
var BeTrue = gomega.BeTrue
var BeFalse = gomega.BeFalse
var HaveOccurred = gomega.HaveOccurred
var Succeed = gomega.Succeed
var MatchError = gomega.MatchError
var BeClosed = gomega.BeClosed
var Receive = gomega.Receive
var BeSent = gomega.BeSent
var MatchRegexp = gomega.MatchRegexp
var ContainSubstring = gomega.ContainSubstring
var HavePrefix = gomega.HavePrefix
var HaveSuffix = gomega.HaveSuffix
var MatchJSON = gomega.MatchJSON
var MatchXML = gomega.MatchXML
var MatchYAML = gomega.MatchYAML
var BeEmpty = gomega.BeEmpty
var HaveLen = gomega.HaveLen
var HaveCap = gomega.HaveCap
var BeZero = gomega.BeZero
var ContainElement = gomega.ContainElement
var BeElementOf = gomega.BeElementOf
var ConsistOf = gomega.ConsistOf
var HaveKey = gomega.HaveKey
var HaveKeyWithValue = gomega.HaveKeyWithValue
var BeNumerically = gomega.BeNumerically
var BeTemporally = gomega.BeTemporally
var BeAssignableToTypeOf = gomega.BeAssignableToTypeOf
var Panic = gomega.Panic
var BeAnExistingFile = gomega.BeAnExistingFile
var BeARegularFile = gomega.BeARegularFile
var BeADirectory = gomega.BeADirectory
var And = gomega.And
var SatisfyAll = gomega.SatisfyAll
var Or = gomega.Or
var SatisfyAny = gomega.SatisfyAny
var Not = gomega.Not
var WithTransform = gomega.WithTransform
