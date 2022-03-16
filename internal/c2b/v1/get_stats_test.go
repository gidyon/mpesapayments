package c2b

import (
	"context"
	"fmt"
	"time"

	"github.com/Pallinder/go-randomdata"
	c2b "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	accountNames = []string{"test1", "test2", "test3"}
	shortCodeX   = int32(4567)
)

func accountName() string {
	return accountNames[randomdata.Number(0, len(accountNames))]
}

var _ = Describe("Getting stats @getstat", func() {
	var (
		ctx    context.Context
		getReq *c2b.GetStatsRequest
	)

	BeforeEach(func() {
		ctx = context.Background()
		getReq = &c2b.GetStatsRequest{
			Dates:       []string{time.Now().String()[:10]},
			ShortCode:   fmt.Sprint(shortCodeX),
			AccountName: accountName(),
		}
	})

	Describe("Getting stats with malformed request", func() {
		It("should fail when the request is nil", func() {
			getReq = nil
			getRes, err := C2BAPI.GetStats(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when short code is missing", func() {
			getReq.ShortCode = ""
			getRes, err := C2BAPI.GetStats(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when account name is missing", func() {
			getReq.AccountName = ""
			getRes, err := C2BAPI.GetStats(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when dates is missing", func() {
			getReq.Dates = nil
			getRes, err := C2BAPI.GetStats(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
	})

	Describe("Getting stats with valid request", func() {
		Context("We create some random transactions", func() {
			It("should always succeed", func() {
				var (
					err   error
					count = 1000
				)
				payments := make([]*PaymentMpesa, 0, 5)
				for i := 0; i < count; i++ {
					paymentDB, err := C2BPaymentDB(fakeC2BPayment())
					Expect(err).ShouldNot(HaveOccurred())

					paymentDB.ReferenceNumber = accountName()
					paymentDB.BusinessShortCode = shortCodeX
					payments = append(payments, paymentDB)
				}
				err = C2BAPIServer.SQLDB.CreateInBatches(payments, count).Error
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Describe("Getting stats", func() {
			It("should succed", func() {
				getRes, err := C2BAPI.GetStats(ctx, getReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
			})
		})
	})
})
