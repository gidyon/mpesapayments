package b2c

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Getting b2c payment @get", func() {
	var (
		getReq *b2c.GetB2CPaymentRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		getReq = &b2c.GetB2CPaymentRequest{
			PaymentId: uuid.New().String(),
		}
		ctx = context.Background()
	})

	When("Getting b2c with malformed request", func() {
		It("should fail when the request is nil", func() {
			getReq = nil
			getRes, err := B2CAPI.GetB2CPayment(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when the payment id is missing", func() {
			getReq.PaymentId = ""
			getRes, err := B2CAPI.GetB2CPayment(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when payment does not exist", func() {
			getReq.PaymentId = fmt.Sprint(randomdata.Number(99999, 999999))
			getRes, err := B2CAPI.GetB2CPayment(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.NotFound))
			Expect(getRes).Should(BeNil())
		})
	})

	Describe("Getting b2c with well-formed request", func() {
		var paymentID string
		Context("Lets create b2c payment first", func() {
			It("should succeed", func() {
				paymentDB, err := B2CPaymentDB(fakeB2CPayment())
				Expect(err).ShouldNot(HaveOccurred())

				err = B2CAPIServer.SQLDB.Create(paymentDB).Error
				Expect(err).ShouldNot(HaveOccurred())

				paymentID = fmt.Sprint(paymentDB.ID)
			})
		})

		Describe("Getting the created b2c payment", func() {
			It("should succeed", func() {
				getReq.PaymentId = paymentID
				getRes, err := B2CAPI.GetB2CPayment(ctx, getReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
			})
		})
	})
})
