package stk

import (
	"context"
	"fmt"

	"github.com/Pallinder/go-randomdata"
	stk "github.com/gidyon/mpesapayments/pkg/api/stk/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Processing stk payload @process", func() {
	var (
		req *stk.ProcessStkTransactionRequest
		ctx context.Context
	)

	BeforeEach(func() {
		req = &stk.ProcessStkTransactionRequest{
			TransactionId: fmt.Sprint(randomdata.Number(99, 999)),
			Processed:     true,
		}
		ctx = context.Background()
	})

	Describe("Processing stk payload with malformed request", func() {
		It("should fail when the request is nil", func() {
			req = nil
			processRes, err := StkAPI.ProcessStkTransaction(ctx, req)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
		It("should fail when transaction id is missing", func() {
			req.TransactionId = ""
			processRes, err := StkAPI.ProcessStkTransaction(ctx, req)
			Expect(err).Should(HaveOccurred())
			Expect(status.Code(err)).Should(Equal(codes.InvalidArgument))
			Expect(processRes).Should(BeNil())
		})
	})

	Describe("Processing request with malformed request", func() {
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

		Describe("Processing the request", func() {
			It("should succeed", func() {
				req.TransactionId = ID
				req.Processed = true
				processRes, err := StkAPI.ProcessStkTransaction(ctx, req)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(processRes).ShouldNot(BeNil())
			})
		})

		Context("Getting the stk payload", func() {
			Specify("processed to be true", func() {
				getRes, err := StkAPI.GetStkTransaction(ctx, &stk.GetStkTransactionRequest{
					TransactionId: ID,
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(status.Code(err)).Should(Equal(codes.OK))
				Expect(getRes).ShouldNot(BeNil())
				Expect(getRes.Processed).Should(BeTrue())
			})
		})
	})
})
