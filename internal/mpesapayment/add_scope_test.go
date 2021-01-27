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
		addReq *mpesapayment.SaveScopesRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		addReq = &mpesapayment.SaveScopesRequest{
			UserId: fmt.Sprint(randomdata.Number(99, 999)),
			Scopes: &mpesapayment.Scopes{
				AllowedAccNumber: []string{randomdata.Adjective(), randomdata.Adjective()},
				AllowedPhones:    []string{randomdata.PhoneNumber(), randomdata.PhoneNumber()},
				AllowedAmounts:   []float32{float32(randomdata.Decimal(10, 1000)), float32(randomdata.Decimal(10, 1000))},
				Percentage:       10,
			},
		}
		ctx = context.Background()
	})

	Describe("Adding scopes with malformed request", func() {
		It("should fail when the request is nil", func() {
			addReq = nil
			addRes, err := MpesaPaymentAPI.SaveScopes(ctx, addReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(addRes).Should(BeNil())
		})
		It("should fail when user is is missing", func() {
			addReq.UserId = ""
			addRes, err := MpesaPaymentAPI.SaveScopes(ctx, addReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(addRes).Should(BeNil())
		})
		It("should fail when scopes is nil", func() {
			addReq.Scopes = nil
			addRes, err := MpesaPaymentAPI.SaveScopes(ctx, addReq)
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
			addRes, err := MpesaPaymentAPI.SaveScopes(ctx, addReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(addRes).ShouldNot(BeNil())
			userID = addReq.UserId
			scopes = addReq.Scopes
		})

		Describe("Getting the scopes", func() {
			It("should succeed", func() {
				getRes, err := MpesaPaymentAPI.GetScopes(ctx, &mpesapayment.GetScopesRequest{
					UserId: userID,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
				Expect(getRes.Scopes.GetAllowedAccNumber()).ShouldNot(BeNil())
				Expect(getRes.Scopes.GetAllowedPhones()).ShouldNot(BeNil())
				// Scopes must be equivalent
				Expect(scopes.Percentage).Should(Equal(getRes.Scopes.Percentage))
				for _, scope := range getRes.Scopes.GetAllowedAccNumber() {
					Expect(scope).Should(BeElementOf(scopes.AllowedAccNumber))
				}
				for _, scope := range getRes.Scopes.GetAllowedPhones() {
					Expect(scope).Should(BeElementOf(scopes.AllowedPhones))
				}
				for _, scope := range getRes.Scopes.GetAllowedAmounts() {
					Expect(scope).Should(BeElementOf(scopes.AllowedAmounts))
				}
			})
		})
	})
})
