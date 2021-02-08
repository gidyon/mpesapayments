package c2b

import (
	"context"
	"fmt"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Publishing an Mpesa Payment @publish", func() {
	var (
		pubReq *c2b.PublishC2BPaymentRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		pubReq = &c2b.PublishC2BPaymentRequest{
			PaymentId: fmt.Sprint(randomdata.Number(99, 999)),
		}
		ctx = context.Background()
	})

	Describe("Publishing mpesa payment with malformed request", func() {
		It("should fail when the request is nil", func() {
			pubReq = nil
			pubRes, err := C2BAPI.PublishC2BPayment(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
		It("should fail when payment id is missing", func() {
			pubReq.PaymentId = ""
			pubRes, err := C2BAPI.PublishC2BPayment(ctx, pubReq)
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
			msgChan := C2BAPIServer.RedisDB.Subscribe(ctx, C2BAPIServer.AddPrefix(C2BAPIServer.PublishChannel)).Channel()

			for msg := range msgChan {
				strs := strings.Split(msg.Payload, ":")
				paymentReceive = strs[1]
				break
			}
			close(ch)
		}

		Context("Lets create mpesa payment first", func() {
			It("should succeed", func() {
				go listenFn()

				paymentDB, err := GetMpesaDB(fakeC2BPayment())
				Expect(err).ShouldNot(HaveOccurred())

				err = C2BAPIServer.SQLDB.Create(paymentDB).Error
				Expect(err).ShouldNot(HaveOccurred())

				paymentID = paymentDB.TransactionID
			})
		})

		Context("Lets publish the mpesa payment", func() {
			It("should succeed", func() {
				pubRes, err := C2BAPI.PublishC2BPayment(ctx, &c2b.PublishC2BPaymentRequest{
					PaymentId:   paymentID,
					InitiatorId: randomdata.RandStringRunes(16),
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
			It("should succeed when processed state is processed", func() {
				pubRes, err := C2BAPI.PublishC2BPayment(ctx, &c2b.PublishC2BPaymentRequest{
					PaymentId:      paymentID,
					InitiatorId:    randomdata.RandStringRunes(16),
					ProcessedState: c2b.ProcessedState_PROCESSED,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
			It("should succeed when processed state is not processed", func() {
				pubRes, err := C2BAPI.PublishC2BPayment(ctx, &c2b.PublishC2BPaymentRequest{
					PaymentId:      paymentID,
					InitiatorId:    randomdata.RandStringRunes(16),
					ProcessedState: c2b.ProcessedState_NOT_PROCESSED,
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
