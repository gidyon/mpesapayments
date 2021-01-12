package mpesapayment

import (
	"context"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Getting random transaction @random", func() {
	var (
		getReq      *mpesapayment.GetRandomTransactionRequest
		ctx         context.Context
		accountsNum = []string{"a", "b", "c", "d"}
	)

	BeforeEach(func() {
		getReq = &mpesapayment.GetRandomTransactionRequest{
			AccountsNumber:   accountsNum,
			StartTimeSeconds: time.Now().Unix() - 1000,
			EndTimeSeconds:   time.Now().Unix(),
		}
		ctx = context.Background()
	})

	Describe("Getting random transaction with malformed request", func() {
		It("should fail when the request is nil", func() {
			getReq = nil
			getRes, err := MpesaPaymentAPI.GetRandomTransaction(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
	})

	Describe("Getting random transaction with well request", func() {
		Describe("Creating random transactions", func() {
			It("should always succeed", func() {
				var err error
				payments := make([]*PaymentMpesa, 0, 50)
				for i := 0; i < 50; i++ {
					paymentDB, err := GetMpesaDB(fakeMpesaPayment())
					Expect(err).ShouldNot(HaveOccurred())

					paymentDB.ReferenceNumber = accountsNum[randomdata.Number(0, len(accountsNum))]
					payments = append(payments, paymentDB)
				}
				err = MpesaPaymentAPIServer.SQLDB.CreateInBatches(payments, 50).Error
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		When("Getting a random transaction", func() {
			It("should succeed when filters is present", func() {
				getRes, err := MpesaPaymentAPI.GetRandomTransaction(ctx, getReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
			})
			It("should succeed when filters is not present", func() {
				getReq = &mpesapayment.GetRandomTransactionRequest{}
				getRes, err := MpesaPaymentAPI.GetRandomTransaction(ctx, getReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
			})
		})
	})
})
