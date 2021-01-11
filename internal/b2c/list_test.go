package b2c

import (
	"context"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/b2c"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing b2c transactions @list", func() {
	var (
		listReq *b2c.ListB2CPaymentsRequest
		ctx     context.Context
	)

	BeforeEach(func() {
		listReq = &b2c.ListB2CPaymentsRequest{
			PageSize: 20,
			Filter:   &b2c.ListB2CPaymentFilter{},
		}
		ctx = context.Background()
	})

	Describe("Listing b2c transactions with malformed request", func() {
		It("should fail when the request is nil", func() {
			listReq = nil
			listRes, err := B2CAPI.ListB2CPayments(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
		It("should fail when filter date is incorrect", func() {
			listReq.Filter.TxDate = time.Now().String()[:11]
			listRes, err := B2CAPI.ListB2CPayments(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Listing b2c transactions with well formed request", func() {
		Context("Lets create random b2c transactions", func() {
			It("should succeed", func() {
				for i := 0; i < 100; i++ {
					paymentDB, err := GetB2CPaymentDB(fakeB2CPayment())
					Expect(err).ShouldNot(HaveOccurred())

					err = B2CAPIServer.SQLDB.Create(paymentDB).Error
					Expect(err).ShouldNot(HaveOccurred())
				}
			})

			Describe("Listing payments", func() {
				var (
					pageToken string
					next      = true
				)
				It("should succeed", func() {
					for next {
						listReq.PageToken = pageToken
						listRes, err := B2CAPI.ListB2CPayments(ctx, listReq)
						Expect(err).ShouldNot(HaveOccurred())
						Expect(status.Code(err)).Should(Equal(codes.OK))
						Expect(listRes).ShouldNot(BeNil())
						pageToken = listRes.NextPageToken
						if pageToken == "" {
							next = false
						}
					}
				})
			})

			Describe("Listing payments with filter", func() {
				It("should succeed", func() {
					listReq.Filter = &b2c.ListB2CPaymentFilter{
						TxDate:  time.Now().String()[:10],
						Msisdns: []string{"345678"},
					}
					listRes, err := B2CAPI.ListB2CPayments(ctx, listReq)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(listRes).ShouldNot(BeNil())
				})
			})
		})
	})
})
