package b2c

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Processing b2c transaction @process", func() {
	var (
		processReq *b2c.ProcessB2CPaymentRequest
		ctx        context.Context
	)

	BeforeEach(func() {
		processReq = &b2c.ProcessB2CPaymentRequest{
			PaymentId: fmt.Sprint(randomdata.Number(99, 999)),
			Processed: true,
		}
		ctx = context.Background()
	})

	Describe("Processing b2c transaction with malformed request", func() {
		It("should fail when the request is nil", func() {
			processReq = nil
			processRes, err := B2CAPI.ProcessB2CPayment(ctx, processReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
		It("should fail when transaction id is missing", func() {
			processReq.PaymentId = ""
			processRes, err := B2CAPI.ProcessB2CPayment(ctx, processReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
	})

	Describe("Processing request with malformed request", func() {
		var paymentID string
		Context("Lets create b2c transaction first", func() {
			It("should succeed", func() {
				paymentDB, err := B2CPaymentDB(fakeB2CPayment())
				Expect(err).ShouldNot(HaveOccurred())

				err = B2CAPIServer.SQLDB.Create(paymentDB).Error
				Expect(err).ShouldNot(HaveOccurred())

				paymentID = fmt.Sprint(paymentDB.ID)
			})
		})

		Describe("Processing the request", func() {
			It("should succeed", func() {
				processReq.PaymentId = paymentID
				processReq.Processed = true
				processRes, err := B2CAPI.ProcessB2CPayment(ctx, processReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(processRes).ShouldNot(BeNil())
			})
		})

		Context("Getting the b2c transaction", func() {
			Specify("processed to be true", func() {
				getRes, err := B2CAPI.GetB2CPayment(ctx, &b2c.GetB2CPaymentRequest{
					PaymentId: paymentID,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
				Expect(getRes.Processed).Should(BeTrue())
			})
		})
	})
})
