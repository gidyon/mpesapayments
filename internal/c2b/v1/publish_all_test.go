package c2b

import (
	"context"
	"time"

	c2b "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Publishing an Mpesa Payment @publishall", func() {
	var (
		pubReq *c2b.PublishAllC2BPaymentsRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		pubReq = &c2b.PublishAllC2BPaymentsRequest{
			StartTimeSeconds: time.Now().Unix() - int64(time.Minute)/1000,
			EndTimeSeconds:   time.Now().Unix(),
		}
		ctx = context.Background()
	})

	Describe("Publishing mpesa payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			pubReq = nil
			pubRes, err := C2BAPI.PublishAllC2BPayments(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
	})

	Describe("Publishing mpesa payment with well-formed request", func() {
		Context("Lets publish the mpesa payment", func() {
			It("should succeed", func() {
				pubRes, err := C2BAPI.PublishAllC2BPayments(ctx, &c2b.PublishAllC2BPaymentsRequest{})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
		})
	})
})
