package mpesapayment

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Adding scopes @addscopes", func() {
	var (
		addReq *mpesapayment.AddScopesRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		addReq = &mpesapayment.AddScopesRequest{
			UserId: fmt.Sprint(randomdata.Decimal(99, 999)),
			Scopes: &mpesapayment.Scopes{
				AllowedAccNumber: []string{randomdata.Adjective(), randomdata.Adjective()},
				AllowedPhones:    []string{randomdata.PhoneNumber(), randomdata.PhoneNumber()},
			},
		}
	})

	Describe("Adding scopes with malformed request", func() {
		It("should fail when the request is nil", func() {
			addReq = nil
			addRes, err := MpesaPaymentAPI.AddScopes(ctx, addReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(addRes).Should(BeNil())
		})
		It("should fail when user is is missing", func() {
			addReq.UserId = ""
			addRes, err := MpesaPaymentAPI.AddScopes(ctx, addReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(addRes).Should(BeNil())
		})
		It("should fail when scopes is nil", func() {
			addReq.Scopes = nil
			addRes, err := MpesaPaymentAPI.AddScopes(ctx, addReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(addRes).Should(BeNil())
		})
	})

	Describe("Adding scopes with well formed request", func() {
		var (
			userID string
			scopes *mpesapayment.Scopes
		)
		It("should succeed", func() {
			addRes, err := MpesaPaymentAPI.AddScopes(ctx, addReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(addRes).ShouldNot(BeNil())
			userID = addReq.UserId
			scopes = addReq.Scopes
		})

		Describe("Getting the scopes", func() {
			It("should succeed", func() {
				listRes, err := MpesaPaymentAPI.GetScopes(ctx, &mpesapayment.GetScopesRequest{
					UserId: userID,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(listRes).ShouldNot(BeNil())
				Expect(listRes.Scopes.GetAllowedAccNumber()).ShouldNot(BeNil())
				Expect(listRes.Scopes.GetAllowedPhones()).ShouldNot(BeNil())
				// Scopes must be equivalent
				for _, scope := range listRes.Scopes.GetAllowedAccNumber() {
					Expect(scope).Should(BeElementOf(scopes.AllowedAccNumber))
				}
				for _, scope := range listRes.Scopes.GetAllowedPhones() {
					Expect(scope).Should(BeElementOf(scopes.AllowedPhones))
				}
			})
		})
	})
})
