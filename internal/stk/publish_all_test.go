package stk

import (
	"context"

	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Publishing an Mpesa Payment @publishall", func() {
	var (
		pubReq *stk.PublishAllStkPayloadRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		pubReq = &stk.PublishAllStkPayloadRequest{
			SinceTimeSeconds: 100,
		}
		ctx = context.Background()
	})

	Describe("Publishing all stk payloads with malformed request", func() {
		It("should fail when the request is nil", func() {
			pubReq = nil
			pubRes, err := StkAPI.PublishAllStkPayload(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
	})

	Describe("Publishing all stk payloads with well-formed request", func() {
		Context("Lets publish the stk payloads", func() {
			It("should succeed", func() {
				pubRes, err := StkAPI.PublishAllStkPayload(ctx, &stk.PublishAllStkPayloadRequest{
					ProcessedState:   mpesapayment.ProcessedState_NOT_PROCESSED,
					SinceTimeSeconds: 100,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
		})
	})
})
