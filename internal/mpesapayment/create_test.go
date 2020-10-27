package mpesapayment

import (
	"context"
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
		TxId:              randomdata.RandStringRunes(32),
		TxType:            txTypes[randomdata.Number(0, len(txTypes))],
		TxTimestamp:       time.Now().Unix(),
		Msisdn:            randomdata.PhoneNumber()[:10],
		Names:             randomdata.SillyName(),
		TxRefNumber:       txBillRefNumbers[randomdata.Number(0, len(txBillRefNumbers))],
		TxAmount:          float32(randomdata.Decimal(1000, 100000)),
		BusinessShortCode: int32(randomdata.Number(1000, 20000)),
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
			createReq.MpesaPayment.TxType = "PAY_BILL"
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
			createReq.MpesaPayment.TxTimestamp = 0
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when tx type is missing", func() {
			createReq.MpesaPayment.TxType = ""
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
		It("should fail when tx amount is missing", func() {
			createReq.MpesaPayment.TxAmount = 0
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(createRes).Should(BeNil())
		})
	})

	Describe("Creating mpesa payment with well formed request", func() {
		It("should succeed", func() {
			createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, createReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(createRes).ShouldNot(BeNil())
		})
	})
})
