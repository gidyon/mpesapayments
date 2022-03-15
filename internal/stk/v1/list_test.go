package stk

import (
	"context"
	"time"

	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing stk payloads @list", func() {
	var (
		req *stk.ListStkTransactionsRequest
		ctx context.Context
	)

	BeforeEach(func() {
		req = &stk.ListStkTransactionsRequest{
			PageSize: 20,
			Filter:   &stk.ListStkTransactionFilter{},
		}
		ctx = context.Background()
	})

	Describe("Listing stk payloads with malformed request", func() {
		It("should fail when the request is nil", func() {
			req = nil
			listRes, err := StkAPI.ListStkTransactions(ctx, req)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
		It("should fail when filter date is incorrect", func() {
			req.Filter.TxDate = time.Now().String()[:11]
			listRes, err := StkAPI.ListStkTransactions(ctx, req)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Listing stk payloads with well formed request", func() {
		Context("Lets create random stk payloads", func() {
			It("should succeed", func() {
				for i := 0; i < 100; i++ {
					createRes, err := StkAPI.CreateStkTransaction(ctx, &stk.CreateStkTransactionRequest{
						Payload: mockStkTransaction(),
					})
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(createRes).ShouldNot(BeNil())

					// DB Direct
					db, err := STKTransactionModel(mockStkTransaction())
					Expect(err).ShouldNot(HaveOccurred())
					err = StkAPIServer.SQLDB.Create(db).Error
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
						req.PageToken = pageToken
						listRes, err := StkAPI.ListStkTransactions(ctx, req)
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
					req.Filter = &stk.ListStkTransactionFilter{
						TxDate:  time.Now().String()[:10],
						Msisdns: []string{"345678"},
					}
					listRes, err := StkAPI.ListStkTransactions(ctx, req)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(listRes).ShouldNot(BeNil())
				})
			})
		})
	})
})
