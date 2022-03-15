package stk

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Getting stk payload @get", func() {
	var (
		getReq *stk.GetStkTransactionRequest
		ctx    context.Context
	)

	BeforeEach(func() {
		getReq = &stk.GetStkTransactionRequest{
			TransactionId: uuid.New().String(),
		}
		ctx = context.Background()
	})

	Describe("Getting stk with malformed request", func() {
		It("should fail when the request is nil", func() {
			getReq = nil
			getRes, err := StkAPI.GetStkTransaction(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when the payload id is missing", func() {
			getReq.TransactionId = ""
			getRes, err := StkAPI.GetStkTransaction(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(getRes).Should(BeNil())
		})
		It("should fail when payment id is missing", func() {
			getReq.TransactionId = fmt.Sprint(randomdata.Number(99999, 999999))
			getRes, err := StkAPI.GetStkTransaction(ctx, getReq)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.NotFound))
			Expect(getRes).Should(BeNil())
		})
	})

	Describe("Getting stk with well-formed request", func() {
		var ID string
		Context("Lets create stk payload first", func() {
			It("should succeed", func() {
				db, err := STKTransactionModel(mockStkTransaction())
				Expect(err).ShouldNot(HaveOccurred())
				err = StkAPIServer.SQLDB.Create(db).Error
				Expect(err).ShouldNot(HaveOccurred())
				ID = fmt.Sprint(db.ID)
			})
		})

		Describe("Getting the created stk payload", func() {
			It("should succeed", func() {
				getReq.TransactionId = ID
				getRes, err := StkAPI.GetStkTransaction(ctx, getReq)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
			})
		})
	})
})
