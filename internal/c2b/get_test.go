package c2b

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Getting mpesa payment @gety", func() {
	var (
		getReq *c2b.GetC2BPaymentRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		getReq = &c2b.GetC2BPaymentRequest{
			PaymentId: fmt.Sprint(randomdata.Number(99, 999)),
		}
		ctx = context.Background()
	})

	Describe("Getting mpesa payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			getReq = nil
			getRes, err := C2BAPI.GetC2BPayment(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when payment id is missing", func() {
			getReq.PaymentId = ""
			getRes, err := C2BAPI.GetC2BPayment(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when payment id does not exist", func() {
			getReq.PaymentId = fmt.Sprint(randomdata.Number(999, 9999))
			getRes, err := C2BAPI.GetC2BPayment(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.NotFound))
			Expect(getRes).Should(BeNil())
		})
	})

	Describe("Getting mpesa payment with well formed request", func() {
		var paymentID string
		Specify("Creating payment first", func() {
			paymentDB, err := C2BPaymentDB(fakeC2BPayment())
			Expect(err).ShouldNot(HaveOccurred())

			err = C2BAPIServer.SQLDB.Create(paymentDB).Error
			Expect(err).ShouldNot(HaveOccurred())

			paymentID = paymentDB.TransactionID
		})

		Context("Getting the payment", func() {
			It("should succeed", func() {
				getRes, err := C2BAPI.GetC2BPayment(ctx, &c2b.GetC2BPaymentRequest{
					PaymentId: paymentID,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
			})
		})
	})
})
