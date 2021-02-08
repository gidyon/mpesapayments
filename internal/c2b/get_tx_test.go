package c2b

import (
	"context"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Getting summary of mpesa transactions @gettx", func() {
	var (
		getReq *c2b.GetTransactionsCountRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		getReq = &c2b.GetTransactionsCountRequest{
			Amounts:        []float32{100},
			AccountsNumber: []string{"a"},
		}
		ctx = context.Background()
	})

	When("Getting transactions summary with malformed request", func() {
		It("should fail when the request is nil", func() {
			getReq = nil
			getRes, err := C2BAPI.GetTransactionsCount(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when the start time is greater the end time", func() {
			getReq.StartTimeSeconds = time.Now().Unix() + 10000
			getReq.EndTimeSeconds = time.Now().Unix()
			getRes, err := C2BAPI.GetTransactionsCount(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
	})

	When("Getting transactions summary with well formed request", func() {
		Context("Lets create some transactions", func() {
			It("should always succeed", func() {
				var err error
				payments := make([]*PaymentMpesa, 0, 50)
				for i := 0; i < 50; i++ {
					paymentDB, err := GetMpesaDB(fakeC2BPayment())
					Expect(err).ShouldNot(HaveOccurred())
					payments = append(payments, paymentDB)
				}
				for i := 0; i < 50; i++ {
					paymentDB, err := GetMpesaDB(fakeC2BPayment())
					Expect(err).ShouldNot(HaveOccurred())
					paymentDB.Amount = 100
					paymentDB.ReferenceNumber = "a"
					payments = append(payments, paymentDB)
				}
				err = C2BAPIServer.SQLDB.CreateInBatches(payments, 50).Error
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("Getting the summary", func() {
			It("should succeed", func() {
				getRes, err := C2BAPI.GetTransactionsCount(ctx, getReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
				C2BAPIServer.Logger.Infoln(getRes)
			})
		})
	})
})
