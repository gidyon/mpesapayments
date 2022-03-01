package stk

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Processing stk payload @process", func() {
	var (
		processReq *stk.ProcessStkPayloadRequest
		ctx        context.Context
	)

	BeforeEach(func() {
		processReq = &stk.ProcessStkPayloadRequest{
			PayloadId: fmt.Sprint(randomdata.Number(99, 999)),
			Processed: true,
		}
		ctx = context.Background()
	})

	Describe("Processing stk payload with malformed request", func() {
		It("should fail when the request is nil", func() {
			processReq = nil
			processRes, err := StkAPI.ProcessStkPayload(ctx, processReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
		It("should fail when payload id is missing", func() {
			processReq.PayloadId = ""
			processRes, err := StkAPI.ProcessStkPayload(ctx, processReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
	})

	Describe("Processing request with malformed request", func() {
		var payloadID string
		Context("Lets create stk payload first", func() {
			It("should succeed", func() {
				payloadDB, err := StkPayloadDB(mockStkPayload())
				Expect(err).ShouldNot(HaveOccurred())
				err = StkAPIServer.SQLDB.Create(payloadDB).Error
				Expect(err).ShouldNot(HaveOccurred())
				payloadID = payloadDB.TransactionID
			})
		})

		Describe("Processing the request", func() {
			It("should succeed", func() {
				processReq.PayloadId = payloadID
				processReq.Processed = true
				processRes, err := StkAPI.ProcessStkPayload(ctx, processReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(processRes).ShouldNot(BeNil())
			})
		})

		Context("Getting the stk payload", func() {
			Specify("processed to be true", func() {
				getRes, err := StkAPI.GetStkPayload(ctx, &stk.GetStkPayloadRequest{
					PayloadId: payloadID,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
				Expect(getRes.Processed).Should(BeTrue())
			})
		})
	})
})
