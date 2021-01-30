package mpesapayment

import (
	"context"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Publishing an Mpesa Payment @publishall", func() {
	var (
		pubReq *mpesapayment.PublishAllMpesaPaymentsRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		pubReq = &mpesapayment.PublishAllMpesaPaymentsRequest{
			StartTimestamp: time.Now().Unix() - int64(time.Minute)/1000,
			EndTimestamp:   time.Now().Unix(),
		}
		ctx = context.Background()
	})

	Describe("Publishing mpesa payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			pubReq = nil
			pubRes, err := MpesaPaymentAPI.PublishAllMpesaPayments(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
	})

	Describe("Publishing mpesa payment with well-formed request", func() {
		Context("Lets publish the mpesa payment", func() {
			It("should succeed", func() {
				pubRes, err := MpesaPaymentAPI.PublishAllMpesaPayments(ctx, &mpesapayment.PublishAllMpesaPaymentsRequest{})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
		})
	})
})
