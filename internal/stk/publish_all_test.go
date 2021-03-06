package stk

import (
	"context"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/c2b"
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
			StartTimestamp: time.Now().Unix() - int64(time.Minute)/1000,
			EndTimestamp:   time.Now().Unix(),
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
					ProcessedState: c2b.ProcessedState_NOT_PROCESSED,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
		})
	})
})
