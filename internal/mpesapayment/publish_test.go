package mpesapayment

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Publishing an Mpesa Payment @publish", func() {
	var (
		pubReq *mpesapayment.PublishMpesaPaymentRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		pubReq = &mpesapayment.PublishMpesaPaymentRequest{
			PaymentId: fmt.Sprint(randomdata.Number(99, 999)),
		}
		ctx = context.Background()
	})

	Describe("Publoshing mpesa payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			pubReq = nil
			pubRes, err := MpesaPaymentAPI.PublishMpesaPayment(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
		It("should fail when payment id is missing", func() {
			pubReq.PaymentId = ""
			pubRes, err := MpesaPaymentAPI.PublishMpesaPayment(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
	})

	Describe("Publishing mpesa payment with well-formed request", func() {
		var (
			paymentID, paymentReceive string
			ch                        = make(chan struct{})
		)

		listenFn := func() {
			msgChan := MpesaPaymentAPIServer.RedisDB.Subscribe(publishChannel).Channel()

			for msg := range msgChan {
				paymentReceive = msg.Payload
				break
			}
			close(ch)
		}

		Context("Lets create mpesa payment first", func() {
			It("should succeed", func() {
				go listenFn()

				createRes, err := MpesaPaymentAPI.CreateMPESAPayment(ctx, &mpesapayment.CreateMPESAPaymentRequest{
					MpesaPayment: fakeMpesaPayment(),
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(createRes).ShouldNot(BeNil())
				paymentID = createRes.PaymentId
			})
		})

		Context("Lets publish the mpesa payment", func() {
			It("should succeed", func() {
				pubRes, err := MpesaPaymentAPI.PublishMpesaPayment(ctx, &mpesapayment.PublishMpesaPaymentRequest{
					PaymentId:      paymentID,
					ProcessedState: mpesapayment.ProcessedState_ANY,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
		})

		Specify("Payment gotten is what was published", func() {
			<-ch
			Expect(paymentReceive).To(Equal(paymentID))
		})
	})

})
