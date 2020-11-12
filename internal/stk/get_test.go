package stk

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Getting stk payload @get", func() {
	var (
		getReq *stk.GetStkPayloadRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		getReq = &stk.GetStkPayloadRequest{
			PayloadId: uuid.New().String(),
		}
		ctx = context.Background()
	})

	Describe("Getting stk with malformed request", func() {
		It("should fail when the request is nil", func() {
			getReq = nil
			getRes, err := StkAPI.GetStkPayload(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when the payload id is missing", func() {
			getReq.PayloadId = ""
			getRes, err := StkAPI.GetStkPayload(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when the payload id is incorrect integer", func() {
			getReq.PayloadId = "dedbibd3d"
			getRes, err := StkAPI.GetStkPayload(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when payment id is missing", func() {
			getReq.PayloadId = fmt.Sprint(randomdata.Number(99999, 999999))
			getRes, err := StkAPI.GetStkPayload(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.NotFound))
			Expect(getRes).Should(BeNil())
		})
	})

	Describe("Getting stk with well-formed request", func() {
		var payloadID string
		Context("Lets create stk payload first", func() {
			It("should succeed", func() {
				createRes, err := StkAPI.CreateStkPayload(ctx, &stk.CreateStkPayloadRequest{
					Payload: mockStkPayload(),
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(createRes).ShouldNot(BeNil())
				payloadID = createRes.PayloadId
			})
		})

		Describe("Getting the created stk payload", func() {
			It("should succeed", func() {
				getReq.PayloadId = payloadID
				getRes, err := StkAPI.GetStkPayload(ctx, getReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
			})
		})
	})
})
