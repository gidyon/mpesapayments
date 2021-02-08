package c2b

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Listing payments @getscopes", func() {
	var (
		listReq *c2b.GetScopesRequest
		ctx     context.Context
	)

	BeforeEach(func() {
		listReq = &c2b.GetScopesRequest{
			UserId: fmt.Sprint(randomdata.Number(99, 999)),
		}
		ctx = context.Background()
	})

	Describe("Getting scopes with mal formed request", func() {
		It("should fail when the request is nil", func() {
			listReq = nil
			listRes, err := C2BAPI.GetScopes(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
		It("should fail when user id is missing", func() {
			listReq.UserId = ""
			listRes, err := C2BAPI.GetScopes(ctx, listReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(listRes).Should(BeNil())
		})
	})

	Describe("Getting scopes with well formed request", func() {
		It("should succeed", func() {
			listRes, err := C2BAPI.GetScopes(ctx, listReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(listRes).ShouldNot(BeNil())
		})
	})
})
