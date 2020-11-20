package stk

import (
	"context"
	"fmt"
	"strings"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/mpesapayment"
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
			msgChan := StkAPIServer.RedisDB.Subscribe(publishChannel).Channel()

			for msg := range msgChan {
				strs := strings.Split(msg.Payload, ":")
				payloadReceive = strs[1]
				break
			}
			close(ch)
		}

		Context("Lets create stk payload first", func() {
			It("should succeed", func() {
				go listenFn()

				createRes, err := StkAPI.CreateStkPayload(ctx, &stk.CreateStkPayloadRequest{
					Payload: mockStkPayload(),
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(createRes).ShouldNot(BeNil())
				payloadID = createRes.PayloadId
			})
		})

		Context("Lets publish the mpesa payload", func() {
			It("should succeed", func() {
				pubRes, err := StkAPI.PublishStkPayload(ctx, &stk.PublishStkPayloadRequest{
					PayloadId:      payloadID,
					ProcessedState: mpesapayment.ProcessedState_ANY,
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
			Expect(payloadReceive).To(Equal(payloadID))
		})
	})

})
