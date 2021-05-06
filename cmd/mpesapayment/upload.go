package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gidyon/micro/v2/pkg/middleware/grpc/auth"
	c2bapp "github.com/gidyon/mpesapayments/internal/c2b"
	"google.golang.org/grpc/metadata"
)

const (
	expectedScheme = "Bearer"
)

func uploadHandler(opt *Options) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		// Method must be POST
		if r.Method != http.MethodPost {
			http.Error(rw, "Only POST method allowed", http.StatusBadRequest)
			return
		}

		referenceName := r.URL.Query().Get("reference_name")

		// Validation
		switch {
		case referenceName == "":
			http.Error(rw, "missing reference id", http.StatusBadRequest)
			return
		default:
			// Maximum upload of 10 MB files
			err := r.ParseMultipartForm(10 << 20)
			if err != nil {
				http.Error(rw, fmt.Sprintf("failed to parse file: %v", err), http.StatusBadRequest)
				return
			}
		}

		// Get jwt
		var token string
		jwtBearer := r.Header.Get("authorization")
		if jwtBearer == "" {
			token = r.URL.Query().Get("token")
		} else {
			splits := strings.SplitN(jwtBearer, " ", 2)
			if len(splits) < 2 {
				http.Error(rw, "bad authorization string", http.StatusBadRequest)
				return
			}
			if !strings.EqualFold(splits[0], expectedScheme) {
				http.Error(rw, "request unauthenticated with "+expectedScheme, http.StatusBadRequest)
				return
			}
			token = splits[1]
		}

		if token == "" {
			http.Error(rw, "missing session token in request", http.StatusBadRequest)
			return
		}

		// Communication context
		ctx := metadata.NewIncomingContext(
			r.Context(), metadata.Pairs(auth.Header(), fmt.Sprintf("%s %s", auth.Scheme(), token)),
		)

		// Authorize the context
		ctx, err := opt.AuthAPI.AuthorizeFunc(ctx)
		if err != nil {
			http.Error(rw, fmt.Sprintf("failed to authorize request: %v", err), http.StatusBadRequest)
			return
		}

		actor, err := opt.AuthAPI.GetJwtPayload(ctx)
		if err != nil {
			http.Error(rw, fmt.Sprintf("failed to get jwt payload: %v", err), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("blast")
		if err != nil {
			http.Error(rw, fmt.Sprintf("failed reading form file: %v", err), http.StatusBadRequest)
			return
		}

		defer file.Close()

		var (
			reader      = csv.NewReader(file)
			totalMsisdn int32
			batchSize   = 1000
			fileDatas   = make([]*c2bapp.UploadedFileData, 0, batchSize)
		)

		// Start transaction
		tx := opt.SQLDB.Begin()
		if tx.Error != nil {
			http.Error(rw, fmt.Sprintf("failed to start transaction: %v", err), http.StatusBadRequest)
			return
		}

		// File object
		blastFileDB := &c2bapp.BlastFile{
			ReferenceName: referenceName,
			FileName:      header.Filename,
			UploaderNames: actor.Names,
			TotalMsisdn:   totalMsisdn,
		}

		// Create blast file in db
		err = tx.Create(blastFileDB).Error
		if err != nil {
			tx.Rollback()
			http.Error(rw, fmt.Sprintf("failed to create blast file: %v", err), http.StatusBadRequest)
			return
		}

		for {
			record, err := reader.Read()
			if err == io.EOF {
				err = tx.CreateInBatches(fileDatas, batchSize+1).Error
				if err != nil {
					tx.Rollback()
					http.Error(rw, fmt.Sprintf("failed to create blast file: %v", err), http.StatusBadRequest)
					return
				}
				break
			}

			if len(record) < 1 {
				opt.Logger.Warningf("skipping row due to incorrect length: got %d expected at least 1", len(record))
				continue
			}

			switch record[0] {
			case "msisdn":
				continue
			case "MSISDN":
				continue
			case "phone number":
				continue
			case "phone_number":
				continue
			case "Phone Number":
				continue
			case "Phone_Number":
				continue
			case "PHONE NUMBER":
				continue
			case "PHONE_NUMBER":
				continue
			}

			// Get phone
			phone, err := strconv.Atoi(strings.TrimSpace(record[0]))
			if err != nil {
				opt.Logger.Warningf("skipping row due to incorrect phone digits %s", record[0])
				continue
			}

			// File data
			fd := &c2bapp.UploadedFileData{
				Msisdn: fmt.Sprint(phone),
				FileID: fmt.Sprint(blastFileDB.ID),
			}

			fileDatas = append(fileDatas, fd)

			if len(fileDatas) >= batchSize {
				err = tx.CreateInBatches(fileDatas, batchSize+1).Error
				if err != nil {
					tx.Rollback()
					http.Error(rw, fmt.Sprintf("failed to create blast file: %v", err), http.StatusBadRequest)
					return
				}
				fileDatas = fileDatas[0:0]
			}

			totalMsisdn++
		}

		// Update blast file in db
		err = tx.Model(&c2bapp.BlastFile{}).Where("id=?", blastFileDB.ID).Update("total_msisdn", totalMsisdn).Error
		if err != nil {
			tx.Rollback()
			http.Error(rw, fmt.Sprintf("failed to update blast file: %v", err), http.StatusBadRequest)
			return
		}

		// Commit transaction
		err = tx.Commit().Error
		if err != nil {
			tx.Rollback()
			http.Error(rw, fmt.Sprintf("failed to commit transaction: %v", err), http.StatusBadRequest)
			return
		}

		_, err = rw.Write([]byte("successfully uploaded"))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
		}
	}
}
