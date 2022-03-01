package stk

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"github.com/gidyon/mpesapayments/pkg/api/stk"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Publishing stk payload @publish", func() {
	var (
		pubReq *stk.PublishStkPayloadRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		pubReq = &stk.PublishStkPayloadRequest{
			PayloadId: fmt.Sprint(randomdata.Number(99, 999)),
		}
		ctx = context.Background()
	})

	Describe("Publishing stk payload with malformed request", func() {
		It("should fail when the request is nil", func() {
			pubReq = nil
			pubRes, err := StkAPI.PublishStkPayload(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
		It("should fail when payload id is missing", func() {
			pubReq.PayloadId = ""
			pubRes, err := StkAPI.PublishStkPayload(ctx, pubReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(pubRes).Should(BeNil())
		})
	})

	Describe("Publishing stk payload with well-formed request", func() {
		var (
			payloadID, payloadReceive string
			ch                        = make(chan struct{})
		)

		listenFn := func() {
			msgChan := StkAPIServer.RedisDB.Subscribe(context.Background(), AddPrefix(StkAPIServer.PublishChannel, StkAPIServer.RedisKeyPrefix)).Channel()

			for msg := range msgChan {
				payloadReceive = msg.Payload
				break
			}
			close(ch)
		}

		Context("Lets create stk payload first", func() {
			It("should succeed", func() {
				go listenFn()

				payloadDB, err := StkPayloadDB(mockStkPayload())
				Expect(err).ShouldNot(HaveOccurred())
				err = StkAPIServer.SQLDB.Create(payloadDB).Error
				Expect(err).ShouldNot(HaveOccurred())
				payloadID = payloadDB.TransactionID
			})
		})

		Context("Lets publish the mpesa payload", func() {
			It("should succeed", func() {
				pubRes, err := StkAPI.PublishStkPayload(ctx, &stk.PublishStkPayloadRequest{
					PayloadId:      payloadID,
					ProcessedState: c2b.ProcessedState_NOT_PROCESSED,
					Payload: map[string]string{
						"service_id": randomdata.RandStringRunes(32),
					},
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(pubRes).ShouldNot(BeNil())
			})
		})

		Specify("Payload gotten is what was published", func() {
			<-ch
			Expect(payloadReceive).To(ContainSubstring(payloadID))
		})
	})

})
