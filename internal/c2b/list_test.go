package c2b

import (
	"context"
	"time"

	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing payments @list", func() {
	var (
		listReq *c2b.ListC2BPaymentsRequest
		ctx     context.Context
	)

	BeforeEach(func() {
		listReq = &c2b.ListC2BPaymentsRequest{
			PageSize: 20,
			Filter:   &c2b.ListC2BPaymentsFilter{},
		}
		ctx = context.Background()
	})

	Describe("Listing payments with malformed request", func() {
		It("should fail when the request is nil", func() {
			listReq = nil
			listRes, err := C2BAPI.ListC2BPayments(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
		It("should fail when filter date is incorrect", func() {
			listReq.Filter.TxDate = time.Now().String()[:11]
			listRes, err := C2BAPI.ListC2BPayments(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Listing payments with well formed request", func() {
		Context("Lets create random payments", func() {
			It("should always succeed", func() {
				var err error
				payments := make([]*PaymentMpesa, 0, 50)
				for i := 0; i < 50; i++ {
					paymentDB, err := C2BPaymentDB(fakeC2BPayment())
					Expect(err).ShouldNot(HaveOccurred())
					payments = append(payments, paymentDB)
				}
				err = C2BAPIServer.SQLDB.CreateInBatches(payments, 50).Error
				Expect(err).ShouldNot(HaveOccurred())
			})

			Describe("Listing payments now", func() {
				var (
					pageToken     string
					nextPageToken = "q"
				)
				It("should succeed", func() {
					for nextPageToken != "" {
						listReq.PageToken = pageToken
						listRes, err := C2BAPI.ListC2BPayments(ctx, listReq)
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
					listReq.Filter = &c2b.ListC2BPaymentsFilter{
						TxDate:         time.Now().String()[:10],
						Msisdns:        []string{"345678"},
						AccountsNumber: []string{"fgh"},
					}
					listRes, err := C2BAPI.ListC2BPayments(ctx, listReq)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(listRes).ShouldNot(BeNil())
				})
			})
		})
	})
})
