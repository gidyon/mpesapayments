package b2c

import (
	"context"
	"time"

	"github.com/Pallinder/go-randomdata"
	b2c "github.com/gidyon/mpesapayments/pkg/api/b2c/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func randParagraph() string {
	r := randomdata.Paragraph()
	if len(r) > 100 {
		return r[:100]
	}
	return r
}

func fakePhoneNumber() int64 {
	return int64(randomdata.Number(254700000000, 254799999999))
}

var _ = Describe("Querying balance from an API @querybalance", func() {
	var (
		queryReq *b2c.QueryAccountBalanceRequest
		ctx      context.Context
	)

	BeforeEach(func() {
		queryReq = &b2c.QueryAccountBalanceRequest{
			IdentifierType: b2c.QueryAccountBalanceRequest_MSISDN,
			PartyA:         fakePhoneNumber(),
			Remarks:        randParagraph(),
			RequestId:      randomdata.RandStringRunes(32),
			InitiatorId:    randomdata.RandStringRunes(24),
		}
		ctx = context.Background()
	})

	When("Querying account balance with malformed request", func() {
		It("should fail when the request is nil", func() {
			queryReq = nil
			queryRes, err := B2CAPI.QueryAccountBalance(ctx, queryReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
		It("should fail when identifier type is missing", func() {
			queryReq.IdentifierType = b2c.QueryAccountBalanceRequest_QUERY_ACCOUNT_UNSPECIFIED
			queryRes, err := B2CAPI.QueryAccountBalance(ctx, queryReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
		It("should fail when party A type is missing", func() {
			queryReq.PartyA = 0
			queryRes, err := B2CAPI.QueryAccountBalance(ctx, queryReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(queryRes).Should(BeNil())
		})
	})

	When("Querying account balance with well-formed request", func() {
		It("should succeed", func() {
			bs, err := proto.Marshal(fakeB2CPayment())
			Expect(err).ShouldNot(HaveOccurred())

			err = B2CAPIServer.RedisDB.Set(ctx, AddPrefix(queryReq.RequestId, B2CAPIServer.RedisKeyPrefix), bs, 10*time.Second).Err()
			Expect(err).ShouldNot(HaveOccurred())

			time.AfterFunc(time.Second, func() {
				B2CAPIServer.unsubcribe(queryReq.RequestId)
			})

			queryRes, err := B2CAPI.QueryAccountBalance(ctx, queryReq)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.OK))
			Expect(queryRes).ShouldNot(BeNil())
		})
	})
})
