package c2b

import (
	"context"

	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ExportPhones(context.Context, *ExportPhonesRequest) (*emptypb.Empty, error)

func (mpesaAPI *mpesaAPIServer) ExportPhones(
	ctx context.Context, exportReq *c2b.ExportPhonesRequest,
) (*emptypb.Empty, error) {
	// Validation
	switch {
	case exportReq == nil:
		return nil, errs.NilObject("export request")
	case exportReq.ExportFilter == nil:
		return nil, errs.NilObject("export filter")
	case exportReq.Comment == "":
		return nil, errs.MissingField("comment")
	case exportReq.Originator == "":
		return nil, errs.MissingField("originator")
	case exportReq.OriginatorId == "":
		return nil, errs.MissingField("originator id")
	}

	// Get unique mpesa payments
	var (
		next               = true
		pageToken          = ""
		pageSize     int32 = 1000
		uniquePhones       = make(map[string]struct{}, 100)
		ok                 = false
	)

	exportReq.ExportFilter.OnlyUnique = true

	for next {
		// Get unique payments
		listRes, err := mpesaAPI.ListC2BPayments(ctx, &c2b.ListC2BPaymentsRequest{
			PageToken: pageToken,
			PageSize:  pageSize,
			Filter:    exportReq.ExportFilter,
		})
		if err != nil {
			return nil, err
		}

		if listRes.NextPageToken != "" {
			pageToken = listRes.NextPageToken
		} else {
			next = false
		}

		bulksQueues := make([]*QueueBulk, 0, pageSize)

		// Get phone numbers
		for _, c2bPayment := range listRes.MpesaPayments {
			if _, ok = uniquePhones[c2bPayment.Msisdn]; ok {
				continue
			}
			// Update map
			uniquePhones[c2bPayment.Msisdn] = struct{}{}

			// Create export data
			bulkQueue := &QueueBulk{
				Originator:   exportReq.Originator,
				OriginatorID: exportReq.OriginatorId,
				Destination:  c2bPayment.Msisdn,
				Message:      exportReq.Comment,
				SMSCID:       "SAFARICOM_KE",
				Processed:    false,
			}

			bulksQueues = append(bulksQueues, bulkQueue)
		}

		// Insert data
		err = mpesaAPI.SQLDB.CreateInBatches(bulksQueues, int(pageSize)).Error
		if err != nil {
			return nil, errs.FailedToSave("bulk queues", err)
		}
	}

	return &emptypb.Empty{}, nil
}
