package stk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func randomParagraph(l int) string {
	par := randomdata.Paragraph()
	if len(par) > l {
		return par[:l]
	}
	return par
}

func mockStkPayload() *stk.StkPayload {
	return &stk.StkPayload{
		MerchantRequestId:    randomdata.RandStringRunes(48),
		CheckoutRequestId:    randomdata.RandStringRunes(44),
		ResultCode:           fmt.Sprint(randomdata.Number(0, 9999)),
		ResultDesc:           randomParagraph(100),
		Amount:               fmt.Sprint(randomdata.Decimal(5, 10)),
		TransactionId:        strings.ToUpper(randomdata.RandStringRunes(32)),
		TransactionTimestamp: time.Now().Unix(),
		PhoneNumber:          randomdata.PhoneNumber()[:10],
	}
}

var _ = Describe("Creating stk payload record @create", func() {
	var (
		createReq *stk.CreateStkPayloadRequest
		ctx       context.Context
	)

	BeforeEach(func() {
		createReq = &stk.CreateStkPayloadRequest{
			Payload: mockStkPayload(),
		}
		ctx = context.Background()
	})

	Describe("Creating stk payload with malformed request", func() {
		It("should fail when the request is nil", func() {
			createReq = nil
			createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when payload is nil", func() {
			createReq.Payload = nil
			createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when the result code is missing", func() {
			createReq.Payload.ResultCode = ""
			createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when the result desc is missing", func() {
			createReq.Payload.ResultDesc = ""
			createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when amount is missing", func() {
			createReq.Payload.Amount = ""
			createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when transaction date is missing", func() {
			createReq.Payload.TransactionTimestamp = 0
			createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when phone number is missing", func() {
			createReq.Payload.PhoneNumber = ""
			createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
	})

	Describe("Creating stk payload with correct body", func() {
		var checkoutID string
		It("should succeed when payload is correct", func() {
			createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(createRes).ShouldNot(BeNil())
			checkoutID = createRes.CheckoutRequestId
		})

		Describe("Creating stk payload of similar transaction", func() {
			It("should succeed because create is indempotent", func() {
				createReq.Payload.CheckoutRequestId = checkoutID
				createRes, err := StkAPI.CreateStkPayload(ctx, createReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(createRes).ShouldNot(BeNil())
			})
		})
	})
})
