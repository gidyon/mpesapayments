package mpesapayment

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Processing MPESA payment @process", func() {
	var (
		processReq *mpesapayment.ProcessMpesaPaymentRequest
		ctx        context.Context
	)

	BeforeEach(func() {
		processReq = &mpesapayment.ProcessMpesaPaymentRequest{
			PaymentId: fmt.Sprint(randomdata.Number(99, 999)),
			State:     true,
		}
		ctx = MpesaPaymentAPIServer.RedisDB.Context()
	})

	Describe("Processing mpesa payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			processReq = nil
			processRes, err := MpesaPaymentAPI.ProcessMpesaPayment(ctx, processReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
		It("should fail when payment id is missing", func() {
			processReq.PaymentId = ""
			processRes, err := MpesaPaymentAPI.ProcessMpesaPayment(ctx, processReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
	})

	Describe("Processing request with malformed request", func() {
		var paymentID string
		Context("Lets create mpesa payment first", func() {
			It("should succeed", func() {
				paymentDB, err := GetMpesaDB(fakeMpesaPayment())
				Expect(err).ShouldNot(HaveOccurred())

				err = MpesaPaymentAPIServer.SQLDB.Create(paymentDB).Error
				Expect(err).ShouldNot(HaveOccurred())

				paymentID = paymentDB.TransactionID
			})
		})

		Describe("Processing the request", func() {
			It("should succeed", func() {
				processReq.PaymentId = paymentID
				processReq.State = true
				processRes, err := MpesaPaymentAPI.ProcessMpesaPayment(ctx, processReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(processRes).ShouldNot(BeNil())
			})
		})

		Context("Getting the payment", func() {
			It("should succeed", func() {
				getRes, err := MpesaPaymentAPI.GetMPESAPayment(ctx, &mpesapayment.GetMPESAPaymentRequest{
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
