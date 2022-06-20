package b2c

import (
	"context"
	"time"

	b2c_model "github.com/gidyon/mpesapayments/internal/b2c"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing transaction stats @liststat", func() {
	var (
		ctx     context.Context
		listReq *b2c.ListDailyStatsRequest
	)

	BeforeEach(func() {
		ctx = context.Background()
		listReq = &b2c.ListDailyStatsRequest{
			PageToken: "",
			PageSize:  20,
			Filter:    &b2c.ListStatsFilter{},
		}
	})

	Describe("Listing for stats for mpesa transactions with malformed request", func() {
		It("should fail when the request", func() {
			listReq = nil
			listRes, err := B2CAPI.ListDailyStats(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Getting stats with well-formed request", func() {
		Context("We create some random transactions", func() {
			It("should always succeed", func() {
				var err error
				payments := make([]*b2c_model.Payment, 0, 50)
				for i := 0; i < 100; i++ {
					paymentDB, err := B2CPaymentDB(fakeB2CPayment())
					Expect(err).ShouldNot(HaveOccurred())

					payments = append(payments, paymentDB)
				}
				err = B2CAPIServer.SQLDB.CreateInBatches(payments, 50).Error
				Expect(err).ShouldNot(HaveOccurred())

				// Lets wait for stats to be generated
				B2CAPIServer.Logger.Infoln("waiting 5sec for worker to calculate statistics")
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
					listRes, err := B2CAPI.ListDailyStats(ctx, listReq)
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
