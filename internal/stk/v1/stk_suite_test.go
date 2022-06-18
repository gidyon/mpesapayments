package stk

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/micro/v2"
	"github.com/gidyon/micro/v2/pkg/conn"
	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	stk_model "github.com/gidyon/mpesapayments/internal/stk"
	c2b "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	redis "github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func TestC2B(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "StkAPI Suite")
}

var (
	StkAPIServer *stkAPIServer
	StkAPI       stk.StkPushAPIServer
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
	os.Setenv("TABLE_PREFIX", "test")

	workerChan = make(chan struct{})
	ctx = context.Background()

	rand.Seed(time.Now().UnixNano())

	// Start real databases
	db, err := startDB()
	Expect(err).ShouldNot(HaveOccurred())

	Expect(db.Migrator().DropTable(stk_model.StkTable)).ShouldNot(HaveOccurred())

	Expect(db.AutoMigrate(&stk_model.STKTransaction{})).ShouldNot(HaveOccurred())

	redisDB := conn.OpenRedisConn(&redis.Options{
		Addr: "localhost:6379",
	})

	logger := micro.NewLogger("C2B_app", zerolog.TraceLevel)

	// Authentication API
	authAPI, err := auth.NewAPI(&auth.Options{
		SigningKey: []byte("awefr"),
		Issuer:     "MPESA API",
		Audience:   "apis",
	})
	Expect(err).ShouldNot(HaveOccurred())

	stkOptions := &OptionsSTK{
		AccessTokenURL:    randomdata.IpV4Address(),
		accessToken:       randomdata.RandStringRunes(32),
		basicToken:        randomdata.RandStringRunes(32),
		ConsumerKey:       randomdata.RandStringRunes(32),
		ConsumerSecret:    randomdata.RandStringRunes(24),
		BusinessShortCode: "174379",
		AccountReference:  randomdata.Adjective(),
		Timestamp:         "3456789",
		PassKey:           randomdata.RandStringRunes(64),
		CallBackURL:       randomdata.IpV4Address(),
		PostURL:           randomdata.IpV4Address(),
		QueryURL:          randomdata.IpV4Address(),
	}

	httpClient := client(1)

	opt := &Options{
		SQLDB:                     db,
		RedisDB:                   redisDB,
		Logger:                    logger,
		AuthAPI:                   authAPI,
		OptionsSTK:                stkOptions,
		HTTPClient:                httpClient,
		UpdateAccessTokenDuration: time.Second * 5,
		WorkerDuration:            time.Second * 10,
	}

	mpesaAPI := c2b.UnimplementedLipaNaMPESAServer{}

	// Create STK API
	StkAPI, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).ShouldNot(HaveOccurred())

	var ok bool
	StkAPIServer, ok = StkAPI.(*stkAPIServer)
	Expect(ok).Should(BeTrue())

	_, err = NewStkAPI(ctx, nil, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.RedisDB = nil
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.RedisDB = redisDB
	opt.SQLDB = nil
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.SQLDB = db
	opt.OptionsSTK = nil
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK = stkOptions
	opt.Logger = nil
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.Logger = logger
	opt.AuthAPI = nil
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.AuthAPI = authAPI
	opt.HTTPClient = nil
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.HTTPClient = httpClient
	opt.OptionsSTK = nil
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK = stkOptions
	opt.OptionsSTK.AccessTokenURL = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK.AccessTokenURL = randomdata.IpV4Address()
	opt.OptionsSTK.BusinessShortCode = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK.BusinessShortCode = "174379"
	opt.OptionsSTK.CallBackURL = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK.CallBackURL = randomdata.IpV4Address()
	opt.OptionsSTK.AccountReference = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK.AccountReference = randomdata.Adjective()
	opt.OptionsSTK.ConsumerKey = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK.ConsumerKey = randomdata.RandStringRunes(32)
	opt.OptionsSTK.ConsumerSecret = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK.ConsumerSecret = randomdata.RandStringRunes(18)
	opt.OptionsSTK.PassKey = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK.PassKey = randomdata.RandStringRunes(64)
	opt.OptionsSTK.PostURL = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())

	opt.OptionsSTK.PostURL = randomdata.IpV4Address()
	opt.OptionsSTK.Timestamp = ""
	_, err = NewStkAPI(ctx, opt, mpesaAPI)
	Expect(err).Should(HaveOccurred())
})

var _ = AfterSuite(func() {
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
