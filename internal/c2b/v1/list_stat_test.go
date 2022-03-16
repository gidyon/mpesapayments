package c2b

import (
	"context"
	"time"

	c2b "github.com/gidyon/mpesapayments/pkg/api/c2b/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing transaction stats @liststat", func() {
	var (
		ctx     context.Context
		listReq *c2b.ListStatsRequest
	)

	BeforeEach(func() {
		ctx = context.Background()
		listReq = &c2b.ListStatsRequest{
			PageToken: "",
			PageSize:  20,
			Filter:    &c2b.ListStatsFilter{},
		}
	})

	Describe("Listing for stats for mpesa transactions with malformed request", func() {
		It("should fail when the request", func() {
			listReq = nil
			listRes, err := C2BAPI.ListStats(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Getting stats with well-formed request", func() {
		Context("We create some random transactions", func() {
			It("should always succeed", func() {
				var err error
				payments := make([]*PaymentMpesa, 0, 50)
				for i := 0; i < 100; i++ {
					paymentDB, err := C2BPaymentDB(fakeC2BPayment())
					Expect(err).ShouldNot(HaveOccurred())

					paymentDB.ReferenceNumber = accountName()
					paymentDB.BusinessShortCode = shortCodeX
					payments = append(payments, paymentDB)
				}
				err = C2BAPIServer.SQLDB.CreateInBatches(payments, 50).Error
				Expect(err).ShouldNot(HaveOccurred())

				// Lets wait for stats to be generated
				C2BAPIServer.Logger.Infoln("waiting 5sec for worker to calculate statistics")
				time.Sleep(5 * time.Second)
			})
		})

		Describe("Listing stats with well-formed request", func() {
			var (
				pageToken     string
				nextPageToken = "q"
			)
			It("should succeed", func() {
				for nextPageToken != "" {
					listReq.PageToken = pageToken
					listRes, err := C2BAPI.ListStats(ctx, listReq)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(listRes).ShouldNot(BeNil())
					nextPageToken = listRes.NextPageToken
					pageToken = listRes.NextPageToken
				}
			})
		})
	})
})
