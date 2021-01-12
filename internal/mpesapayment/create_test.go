package mpesapayment

import (
	"context"
	"strings"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	txTypes          = []string{"PAY_BILL", "BUY_GOODS"}
	txBillRefNumbers = []string{"abc", "dec", "fgh"}
)

func fakeMpesaPayment() *mpesapayment.MPESAPayment {
	return &mpesapayment.MPESAPayment{
		TransactionId:        strings.ToUpper(randomdata.RandStringRunes(32)),
		TransactionType:      txTypes[randomdata.Number(0, len(txTypes))],
		TransactionTimestamp: time.Now().Unix(),
		Msisdn:               randomdata.PhoneNumber()[:10],
		Names:                randomdata.SillyName(),
		RefNumber:            txBillRefNumbers[randomdata.Number(0, len(txBillRefNumbers))],
		Amount:               float32(randomdata.Decimal(1000, 100000)),
		BusinessShortCode:    int32(randomdata.Number(1000, 20000)),
	}
}

var _ = Describe("Creating MPESA payment @create", func() {
	var (
		createReq *mpesapayment.CreateMPESAPaymentRequest
		ctx       context.Context
	)

	BeforeEach(func() {
		createReq = &mpesapayment.CreateMPESAPaymentRequest{
			MpesaPayment: fakeMpesaPayment(),
		}
		ctx = context.Background()
	})

	Describe("Creating payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			createReq = nil
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when mpesa payment is nil", func() {
			createReq.MpesaPayment = nil
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when short code is missing and tx type is PAY_BILL", func() {
			createReq.MpesaPayment.TransactionType = "PAY_BILL"
			createReq.MpesaPayment.BusinessShortCode = 0
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when msisdn is missing", func() {
			createReq.MpesaPayment.Msisdn = ""
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when tx time is missing", func() {
			createReq.MpesaPayment.TransactionTimestamp = 0
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when tx type is missing", func() {
			createReq.MpesaPayment.TransactionType = ""
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when tx amount is missing", func() {
			createReq.MpesaPayment.Amount = 0
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when tx reference number is missing", func() {
			createReq.MpesaPayment.RefNumber = ""
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when tx id is missing", func() {
			createReq.MpesaPayment.TransactionId = ""
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
	})

	Describe("Creating mpesa payment with well formed request", func() {
		var TransactionID string
		It("should succeed", func() {
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(createRes).ShouldNot(BeNil())
			TransactionID = createReq.MpesaPayment.TransactionId
		})

		Describe("Creating duplicate transaction", func() {
			It("should succeed since create is indepotent", func() {
				createReq.MpesaPayment.TransactionId = TransactionID
				createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(createRes).ShouldNot(BeNil())
			})
		})
	})
})
