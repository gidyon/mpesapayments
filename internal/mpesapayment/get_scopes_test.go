package mpesapayment

import (
	"context"
	"fmt"

	"bitbucket.org/gideonkamau/mpesa-tracking-portal/pkg/api/mpesapayment"
	"github.com/Pallinder/go-randomdata"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing payments @getscopes", func() {
	var (
		listReq *mpesapayment.GetScopesRequest
		ctx     context.Context
	)

	BeforeEach(func() {
		listReq = &mpesapayment.GetScopesRequest{
			UserId: fmt.Sprint(randomdata.Number(99, 999)),
		}
		ctx = context.Background()
	})

	Describe("Listing scopes with mal formed request", func() {
		It("should fail when the request is nil", func() {
			listReq = nil
			listRes, err := MpesaPaymentAPI.GetScopes(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Listing scopes with well formed request", func() {
		It("should succeed", func() {
			listRes, err := MpesaPaymentAPI.GetScopes(ctx, listReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(listRes).ShouldNot(BeNil())
		})
	})
})
