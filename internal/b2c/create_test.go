package b2c

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/b2c"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func fakeTransactionID() string {
	return strings.ToUpper(randomdata.Alphanumeric(10))
}

func fakeB2CPayment() *b2c.B2CPayment {
	phone := fakePhoneNumber()
	return &b2c.B2CPayment{
		OrgShortCode:             "174379",
		Msisdn:                   fmt.Sprint(phone),
		ReceiverPartyPublicName:  fmt.Sprintf("%d - %s", phone, randomdata.SillyName()),
		TransactionType:          "Withdrawal",
		TransactionId:            fakeTransactionID(),
		ConversationId:           randomdata.RandStringRunes(32),
		OriginatorConversationId: randomdata.RandStringRunes(32),
		ResultCode:               "0",
		ResultDescription:        randParagraph(),
		TransactionTimestamp:     time.Now().Unix(),
		CreateTimestamp:          time.Now().Unix(),
		Amount:                   float32(randomdata.Decimal(999, 99999)),
		WorkingAccountFunds:      float32(randomdata.Decimal(99999, 999999)),
		UtilityAccountFunds:      float32(randomdata.Decimal(99999, 999999)),
		ChargesPaidFunds:         float32(randomdata.Decimal(10, 999)),
		RecipientRegistered:      time.Now().Unix()%2 == 0,
		Succeeded:                time.Now().Unix()%2 == 0,
		Processed:                time.Now().Unix()%2 == 0,
	}
}

var _ = Describe("Creating B2CPayment @create", func() {
	var (
		createReq *b2c.CreateB2CPaymentRequest
		ctx       context.Context
	)

	BeforeEach(func() {
		createReq = &b2c.CreateB2CPaymentRequest{
			Payment: fakeB2CPayment(),
			Publish: true,
		}
		ctx = context.Background()
	})

	When("Creating b2c payment with malformed request", func() {
		It("should fail when request is nil", func() {
			createReq = nil
			createdRes, err := B2CAPI.CreateB2CPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createdRes).Should(BeNil())
		})
		It("should fail when payment is nil", func() {
			createReq.Payment = nil
			createdRes, err := B2CAPI.CreateB2CPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createdRes).Should(BeNil())
		})
		It("should fail when receiver public name is missing", func() {
			createReq.Payment.ReceiverPartyPublicName = ""
			createdRes, err := B2CAPI.CreateB2CPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createdRes).Should(BeNil())

		})
		It("should fail when transaction description is missing", func() {
			createReq.Payment.ResultDescription = ""
			createdRes, err := B2CAPI.CreateB2CPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createdRes).Should(BeNil())
		})
		It("should fail when transaction timestamp is missing", func() {
			createReq.Payment.TransactionTimestamp = 0
			createdRes, err := B2CAPI.CreateB2CPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createdRes).Should(BeNil())
		})
	})

	When("Creating b2c payment with well formed request", func() {
		It("should succeed", func() {
			createdRes, err := B2CAPI.CreateB2CPayment(ctx, createReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(createdRes).ShouldNot(BeNil())
		})
	})
})
