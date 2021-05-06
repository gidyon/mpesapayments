package b2c

import (
	"context"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/gidyon/mpesapayments/pkg/api/b2c"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var _ = Describe("Transfering funds @transferfunds", func() {
	var (
		transferReq *b2c.TransferFundsRequest
		ctx         context.Context
	)

	BeforeEach(func() {
		transferReq = &b2c.TransferFundsRequest{
			Amount:    float32(randomdata.Decimal(9, 99999)),
			Msisdn:    fakePhoneNumber(),
			ShortCode: 174379,
			Remarks:   randParagraph(),
			Occassion: randParagraph(),
			CommandId: b2c.TransferFundsRequest_BUSINESS_PAYMENT,
			RequestId: randomdata.RandStringRunes(32),
		}
		ctx = context.Background()
	})

	When("Transferring funds with malformed request", func() {
		It("should fail when the request is nil", func() {
			transferReq = nil
			queryRes, err := B2CAPI.TransferFunds(ctx, transferReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
		It("should fail when amount is missing", func() {
			transferReq.Amount = 0
			queryRes, err := B2CAPI.TransferFunds(ctx, transferReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
		It("should fail when msisdn is missing", func() {
			transferReq.Msisdn = 0
			queryRes, err := B2CAPI.TransferFunds(ctx, transferReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
		It("should fail when command id is missing", func() {
			transferReq.CommandId = b2c.TransferFundsRequest_COMMANDID_UNSPECIFIED
			queryRes, err := B2CAPI.TransferFunds(ctx, transferReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
		It("should fail when short code is missing", func() {
			transferReq.ShortCode = 0
			queryRes, err := B2CAPI.TransferFunds(ctx, transferReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
		It("should fail when remarks is missing", func() {
			transferReq.Remarks = ""
			queryRes, err := B2CAPI.TransferFunds(ctx, transferReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
	})

	When("Transfering funds with well formed request", func() {
		It("should succeed", func() {
			bs, err := proto.Marshal(fakeB2CPayment())
			Expect(err).ShouldNot(HaveOccurred())

			err = B2CAPIServer.RedisDB.Set(ctx, AddPrefix(transferReq.RequestId, B2CAPIServer.RedisKeyPrefix), bs, 10*time.Second).Err()
			Expect(err).ShouldNot(HaveOccurred())

			time.AfterFunc(time.Second, func() {
				B2CAPIServer.unsubcribe(transferReq.RequestId)
			})

			queryRes, err := B2CAPI.TransferFunds(ctx, transferReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(queryRes).ShouldNot(BeNil())
		})
	})
})
