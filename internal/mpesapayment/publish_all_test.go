package mpesapayment

import (
	"context"

	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Publishing an Mpesa Payment @publishall", func() {
	var (
		pubReq *mpesapayment.PublishAllMpesaPaymentRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		pubReq = &mpesapayment.PublishAllMpesaPaymentRequest{
			SinceTimeSeconds: 100,
		}
		ctx = context.Background()
	})

	Describe("Publishing mpesa payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			pubReq = nil
			pubRes, err := MpesaPaymentAPI.PublishAllMpesaPayment(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
	})

	Describe("Publishing mpesa payment with well-formed request", func() {
		Context("Lets publish the mpesa payment", func() {
			It("should succeed", func() {
				pubRes, err := MpesaPaymentAPI.PublishAllMpesaPayment(ctx, &mpesapayment.PublishAllMpesaPaymentRequest{
					ProcessedState:   mpesapayment.ProcessedState_ANY,
					SinceTimeSeconds: 100,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
		})
	})
})
