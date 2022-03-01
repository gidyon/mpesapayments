package c2b

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Processing MPESA payment @process", func() {
	var (
		processReq *c2b.ProcessC2BPaymentRequest
		ctx        context.Context
	)

	BeforeEach(func() {
		processReq = &c2b.ProcessC2BPaymentRequest{
			PaymentId: fmt.Sprint(randomdata.Number(99, 999)),
			State:     true,
		}
		ctx = C2BAPIServer.RedisDB.Context()
	})

	Describe("Processing mpesa payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			processReq = nil
			processRes, err := C2BAPI.ProcessC2BPayment(ctx, processReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
		It("should fail when payment id is missing", func() {
			processReq.PaymentId = ""
			processRes, err := C2BAPI.ProcessC2BPayment(ctx, processReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
	})

	Describe("Processing request with malformed request", func() {
		var paymentID string
		Context("Lets create mpesa payment first", func() {
			It("should succeed", func() {
				paymentDB, err := C2BPaymentDB(fakeC2BPayment())
				Expect(err).ShouldNot(HaveOccurred())

				err = C2BAPIServer.SQLDB.Create(paymentDB).Error
				Expect(err).ShouldNot(HaveOccurred())

				paymentID = paymentDB.TransactionID
			})
		})

		Describe("Processing the request", func() {
			It("should succeed", func() {
				processReq.PaymentId = paymentID
				processReq.State = true
				processRes, err := C2BAPI.ProcessC2BPayment(ctx, processReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(processRes).ShouldNot(BeNil())
			})
		})

		Context("Getting the payment", func() {
			It("should succeed", func() {
				getRes, err := C2BAPI.GetC2BPayment(ctx, &c2b.GetC2BPaymentRequest{
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
