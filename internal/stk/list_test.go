package stk

import (
	"context"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing stk payloads @list", func() {
	var (
		listReq *stk.ListStkPayloadsRequest
		ctx     context.Context
	)

	BeforeEach(func() {
		listReq = &stk.ListStkPayloadsRequest{
			PageSize: 20,
			Filter:   &stk.ListStkPayloadFilter{},
		}
		ctx = context.Background()
	})

	Describe("Listing stk payloads with malformed request", func() {
		It("should fail when the request is nil", func() {
			listReq = nil
			listRes, err := StkAPI.ListStkPayloads(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
		It("should fail when filter date is incorrect", func() {
			listReq.Filter.TxDate = time.Now().String()[:11]
			listRes, err := StkAPI.ListStkPayloads(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Listing stk payloads with well formed request", func() {
		Context("Lets create random stk payloads", func() {
			It("should succeed", func() {
				for i := 0; i < 100; i++ {
					createRes, err := StkAPI.CreateStkPayload(ctx, &stk.CreateStkPayloadRequest{
						Payload: mockStkPayload(),
					})
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(createRes).ShouldNot(BeNil())

					// DB Direct
					payloadDB, err := StkPayloadDB(mockStkPayload())
					Expect(err).ShouldNot(HaveOccurred())
					err = StkAPIServer.SQLDB.Create(payloadDB).Error
					Expect(err).ShouldNot(HaveOccurred())
				}
			})

			Describe("Listing payments now", func() {
				var (
					pageToken     string
					nextPageToken = ""
				)
				It("should succeed", func() {
					for nextPageToken != "" {
						listReq.PageToken = pageToken
						listRes, err := StkAPI.ListStkPayloads(ctx, listReq)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(status.Code(err)).Should(Equal(codes.OK))
						Expect(listRes).ShouldNot(BeNil())
						nextPageToken = listRes.NextPageToken
						pageToken = listRes.NextPageToken
					}
				})
			})

			Describe("Listing payments with filter on", func() {
				It("should succeed", func() {
					listReq.Filter = &stk.ListStkPayloadFilter{
						TxDate:  time.Now().String()[:10],
						Msisdns: []string{"345678"},
					}
					listRes, err := StkAPI.ListStkPayloads(ctx, listReq)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(listRes).ShouldNot(BeNil())
				})
			})
		})
	})
})
