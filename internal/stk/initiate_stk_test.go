package stk

import (
	"context"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Initiating stk request @initiate", func() {
	var (
		initReq *stk.InitiateSTKPushRequest
		ctx     context.Context
	)

	BeforeEach(func() {
		initReq = &stk.InitiateSTKPushRequest{
			Phone:  randomdata.PhoneNumber()[:10],
			Amount: randomdata.Decimal(5, 10),
			Payload: map[string]string{
				"client_id": randomdata.RandStringRunes(32),
			},
		}
		ctx = context.Background()
	})

	Describe("Initiating stk push with malformed request", func() {
		It("should fail when the request is nil", func() {
			initReq = nil
			initRes, err := StkAPI.InitiateSTKPush(ctx, initReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(initRes).Should(BeNil())
		})
		It("should fail when stk payload", func() {
			initReq.Payload = nil
			initRes, err := StkAPI.InitiateSTKPush(ctx, initReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(initRes).Should(BeNil())
		})
		It("should fail when phone is missing", func() {
			initReq.Phone = ""
			initRes, err := StkAPI.InitiateSTKPush(ctx, initReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(initRes).Should(BeNil())
		})
		It("should fail when amount is zero or less", func() {
			initReq.Amount = 0
			initRes, err := StkAPI.InitiateSTKPush(ctx, initReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(initRes).Should(BeNil())
		})
	})

	Describe("Initiating stk push with well-formed request", func() {
		var phoneNumber string
		It("should succeed when the request is valid", func() {
			initRes, err := StkAPI.InitiateSTKPush(ctx, initReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(initRes).ShouldNot(BeNil())
			Expect(initRes.Progress).Should(BeTrue())
			phoneNumber = initReq.Phone
		})
		It("should succeed with progress false when there is another transaction underway", func() {
			initReq.Phone = phoneNumber
			initRes, err := StkAPI.InitiateSTKPush(ctx, initReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(initRes).ShouldNot(BeNil())
			Expect(initRes.Progress).Should(BeFalse())
		})
	})
})
