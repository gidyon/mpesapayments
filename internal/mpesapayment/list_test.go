package mpesapayment

import (
	"context"
	"time"

	"bitbucket.org/gideonkamau/mpesa-tracking-portal/pkg/api/mpesapayment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing payments @list", func() {
	var (
		listReq *mpesapayment.ListMPESAPaymentsRequest
		ctx     context.Context
	)

	BeforeEach(func() {
		listReq = &mpesapayment.ListMPESAPaymentsRequest{
			PageSize: 20,
			Filter:   &mpesapayment.ListMPESAPaymentsFilter{},
		}
		ctx = context.Background()
	})

	Describe("Listing payments with malformed request", func() {
		It("should fail when the request is nil", func() {
			listReq = nil
			listRes, err := MpesaPaymentAPI.ListMPESAPayments(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
		It("should fail when filter date is incorrect", func() {
			listReq.Filter.TxDate = time.Now().String()[:11]
			listRes, err := MpesaPaymentAPI.ListMPESAPayments(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Listing payments with well formed request", func() {
		Context("Lets create random payments", func() {
			It("should succeed", func() {
				for i := 0; i < 100; i++ {
					createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, &mpesapayment.CreateMPESAPaymentRequest{
						MpesaPayment: fakeMpesaPayment(),
					})
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(createRes).ShouldNot(BeNil())
				}
			})

			Describe("Listing payments now", func() {
				var (
					pageToken     string
					nextPageToken = "q"
				)
				It("should succeed", func() {
					for nextPageToken != "" {
						listReq.PageToken = pageToken
						listRes, err := MpesaPaymentAPI.ListMPESAPayments(ctx, listReq)
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
					listReq.Filter = &mpesapayment.ListMPESAPaymentsFilter{
						TxDate:         time.Now().String()[:10],
						Msisdns:        []string{"345678"},
						AccountsNumber: []string{"fgh"},
					}
					listRes, err := MpesaPaymentAPI.ListMPESAPayments(ctx, listReq)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(status.Code(err)).Should(Equal(codes.OK))
					Expect(listRes).ShouldNot(BeNil())
				})
			})
		})
	})
})
