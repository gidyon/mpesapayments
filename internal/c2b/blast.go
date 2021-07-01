package c2b

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/gorm"
)

func (c2bAPI *c2bAPIServer) BlastPhones(
	ctx context.Context, blastReq *c2b.BlastPhonesRequest,
) (*emptypb.Empty, error) {
	// Get initiator
	initiator, err := c2bAPI.AuthAPI.GetJwtPayload(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case blastReq == nil:
		return nil, errs.NilObject("export request")
	case blastReq.ExportFilter == nil:
		return nil, errs.NilObject("export filter")
	case blastReq.Comment == "":
		return nil, errs.MissingField("comment")
	case blastReq.Originator == "":
		return nil, errs.MissingField("originator")
	case blastReq.OriginatorId == "":
		return nil, errs.MissingField("originator id")
	}

	// Get unique mpesa payments
	var (
		next               = true
		ok                 = false
		pageToken          = ""
		pageSize     int32 = 1000
		uniquePhones       = make(map[string]struct{}, 100)
		bulksQueues        = make([]*QueueBulk, 0, pageSize)
	)

	blastReq.ExportFilter.OnlyUnique = true

	for next {
		// Get unique payments
		listRes, err := c2bAPI.ListC2BPayments(ctx, &c2b.ListC2BPaymentsRequest{
			PageToken: pageToken,
			PageSize:  pageSize,
			Filter:    blastReq.ExportFilter,
		})
		if err != nil {
			return nil, err
		}

		if listRes.NextPageToken != "" {
			pageToken = listRes.NextPageToken
		} else {
			next = false
		}

		// Get phone numbers
		for _, c2bPayment := range listRes.MpesaPayments {
			if _, ok = uniquePhones[c2bPayment.Msisdn]; ok {
				continue
			}

			// Update map
			uniquePhones[c2bPayment.Msisdn] = struct{}{}

			// Create export data
			bulkQueue := &QueueBulk{
				Originator:       blastReq.Originator,
				Destination:      c2bPayment.Msisdn,
				Message:          strings.ReplaceAll(blastReq.Comment, `\n`, "\n"),
				SMSCID:           "SAFARICOM_KE",
				MessageDirection: "OUT",
			}

			bulksQueues = append(bulksQueues, bulkQueue)
		}

		// Insert data
		err = c2bAPI.SQLDB.CreateInBatches(bulksQueues, int(pageSize)).Error
		if err != nil {
			return nil, errs.FailedToSave("bulk queues", err)
		}

		bulksQueues = bulksQueues[0:0]
	}

	// Add report
	blastReport := &BlastReport{
		Originator:    blastReq.Originator,
		Initiator:     fmt.Sprintf("%s - %s", initiator.Names, initiator.ID),
		Source:        "FILTER",
		Message:       blastReq.Comment,
		TotalExported: int32(len(uniquePhones)),
	}

	// Create in db
	err = c2bAPI.SQLDB.Create(blastReport).Error
	if err != nil {
		c2bAPI.Logger.Errorf("failed to save export report: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (c2bAPI *c2bAPIServer) BlastPhonesFromFile(
	ctx context.Context, blastReq *c2b.BlastPhonesFromFileRequest,
) (*emptypb.Empty, error) {
	// Get initiator
	initiator, err := c2bAPI.AuthAPI.GetJwtPayload(ctx)
	if err != nil {
		return nil, err
	}

	// Validation
	switch {
	case blastReq == nil:
		return nil, errs.NilObject("export request")
	case blastReq.FileId == "":
		return nil, errs.MissingField("file id")
	case blastReq.Comment == "":
		return nil, errs.MissingField("comment")
	case blastReq.Originator == "":
		return nil, errs.MissingField("originator")
	case blastReq.OriginatorId == "":
		return nil, errs.MissingField("originator id")
	case blastReq.EndIndex < blastReq.StartIndex:
		return nil, errs.WrapMessage(codes.InvalidArgument, "end index smaller than start index")
	}

	// Get blast file
	blastFileDB := &BlastFile{}
	err = c2bAPI.SQLDB.First(blastFileDB, "id=?", blastReq.FileId).Error
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errs.DoesNotExist("blast file", blastReq.FileId)
	default:
		return nil, errs.FailedToFind("blast file", err)
	}

	if blastFileDB.TotalMsisdn < blastReq.EndIndex {
		return nil, errs.WrapMessagef(codes.InvalidArgument, "end index %d is larger than file size %d", blastReq.EndIndex, blastFileDB.TotalMsisdn)
	}

	// Get blast data with highest id
	fileDataFirst := &UploadedFileData{}
	err = c2bAPI.SQLDB.Where("file_id = ?", blastFileDB.ID).Limit(1).Order("id DESC").First(fileDataFirst).Error
	switch {
	case err == nil:
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, errs.DoesNotExist("blast file data", blastReq.FileId)
	default:
		return nil, errs.FailedToFind("blast file data", err)
	}

	// Get all phones
	var (
		count        = 0
		bulkSize     = 1000
		next         = true
		ok           = false
		dataID       = fileDataFirst.ID
		fileDatas    = make([]*UploadedFileData, 0, bulkSize+1)
		bulksQueues  = make([]*QueueBulk, 0, bulkSize)
		uniquePhones = make(map[string]struct{}, bulkSize)
	)

	for next {
		db := c2bAPI.SQLDB.Limit(bulkSize+1).Order("id DESC").Where("file_id = ?", blastFileDB.ID)
		if blastReq.EndIndex > 0 {
			diff := fileDataFirst.ID - uint(blastReq.EndIndex)
			endIndex := blastReq.EndIndex + int32(diff)
			startIndex := blastReq.StartIndex + int32(diff)
			db = db.Where("id BETWEEN ? AND ?", startIndex+1, endIndex)
		}
		if dataID > 0 {
			db = db.Where("id <= ?", dataID)
		}

		err = db.Find(&fileDatas).Error
		if err != nil {
			return nil, errs.FailedToFind("blast file data", err)
		}

		for _, fileData := range fileDatas {
			if _, ok = uniquePhones[fileData.Msisdn]; ok {
				continue
			}

			// Update map
			uniquePhones[fileData.Msisdn] = struct{}{}

			// Create export data
			bulkQueue := &QueueBulk{
				Originator:       blastReq.Originator,
				Destination:      fileData.Msisdn,
				Message:          strings.ReplaceAll(blastReq.Comment, `\n`, "\n"),
				SMSCID:           "SAFARICOM_KE",
				MessageDirection: "OUT",
			}

			bulksQueues = append(bulksQueues, bulkQueue)
			count++

			dataID = fileData.ID
		}

		if len(fileDatas) <= bulkSize {
			next = false
		}

		// Insert data
		err = c2bAPI.SQLDB.CreateInBatches(bulksQueues, int(bulkSize+1)).Error
		if err != nil {
			return nil, errs.FailedToSave("bulk queues", err)
		}

		bulksQueues = bulksQueues[0:0]
		fileDatas = fileDatas[0:0]
	}

	// Add report
	blastReport := &BlastReport{
		Originator:    blastReq.Originator,
		Initiator:     fmt.Sprintf("%s - %s", initiator.Names, initiator.ID),
		Source:        "FILE",
		BlastFile:     fmt.Sprintf("%s:%s", blastReq.FileId, blastFileDB.ReferenceName),
		Message:       blastReq.Comment,
		TotalExported: int32(len(uniquePhones)),
	}

	// Save blast report
	err = c2bAPI.SQLDB.Create(blastReport).Error
	if err != nil {
		c2bAPI.Logger.Errorf("failed to save export report: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (c2bAPI *c2bAPIServer) ListBlastReports(
	ctx context.Context, listReq *c2b.ListBlastReportsRequest,
) (*c2b.ListBlastReportsResponse, error) {
	var (
		pageSize  = listReq.GetPageSize()
		pageToken = listReq.GetPageToken()
		paymentID uint
		err       error
	)

	if pageSize <= 0 || pageSize > defaultPageSize {
		pageSize = defaultPageSize
	}

	if pageToken != "" {
		ids, err := c2bAPI.PaginationHasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		paymentID = uint(ids[0])
	}

	blastReports := make([]*BlastReport, 0, pageSize+1)

	db := c2bAPI.SQLDB.Limit(int(pageSize + 1)).Order("id DESC")

	// Apply payment id filter
	if paymentID != 0 {
		db = db.Where("id<?", paymentID)
	}

	err = db.Find(&blastReports).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("ussd channels", err)
	}

	exportsPB := make([]*c2b.BlastReport, 0, len(blastReports))

	for i, paymentDB := range blastReports {
		paymentPaymenPB, err := GetExportPB(paymentDB)
		if err != nil {
			return nil, err
		}

		// Ignore the last element
		if i == int(pageSize) {
			break
		}

		exportsPB = append(exportsPB, paymentPaymenPB)
		paymentID = paymentDB.ID
	}

	var token string
	if len(blastReports) > int(pageSize) {
		// Next page token
		token, err = c2bAPI.PaginationHasher.EncodeInt64([]int64{int64(paymentID)})
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to generate next page token")
		}
	}

	return &c2b.ListBlastReportsResponse{
		BlastReports:  exportsPB,
		NextPageToken: token,
	}, nil
}

func (c2bAPI *c2bAPIServer) ListBlastFiles(
	ctx context.Context, listReq *c2b.ListBlastFilesRequest,
) (*c2b.ListBlastFilesResponse, error) {
	var (
		pageSize  = listReq.GetPageSize()
		pageToken = listReq.GetPageToken()
		blastID   uint
		err       error
	)

	if pageSize <= 0 || pageSize > defaultPageSize {
		pageSize = defaultPageSize
	}

	if pageToken != "" {
		ids, err := c2bAPI.PaginationHasher.DecodeInt64WithError(listReq.GetPageToken())
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to parse page token")
		}
		blastID = uint(ids[0])
	}

	db := c2bAPI.SQLDB.Limit(int(pageSize + 1)).Order("id DESC").Debug()

	// Apply payment id filter
	if blastID != 0 {
		db = db.Where("id<?", blastID)
	}

	blastDBs := make([]*BlastFile, 0, pageSize+1)

	err = db.Find(&blastDBs).Error
	switch {
	case err == nil:
	default:
		return nil, errs.FailedToFind("ussd channels", err)
	}

	blastPBs := make([]*c2b.BlastFile, 0, len(blastDBs))

	for i, blastFileDB := range blastDBs {
		blastFilePB, err := GetBlastFilePB(blastFileDB)
		if err != nil {
			return nil, err
		}

		// Ignore the last element
		if i == int(pageSize) {
			break
		}

		blastPBs = append(blastPBs, blastFilePB)
		blastID = blastFileDB.ID
	}

	var token string
	if len(blastDBs) > int(pageSize) {
		// Next page token
		token, err = c2bAPI.PaginationHasher.EncodeInt64([]int64{int64(blastID)})
		if err != nil {
			return nil, errs.WrapErrorWithCodeAndMsg(codes.InvalidArgument, err, "failed to generate next page token")
		}
	}

	return &c2b.ListBlastFilesResponse{
		BlastFiles:    blastPBs,
		NextPageToken: token,
	}, nil
}
