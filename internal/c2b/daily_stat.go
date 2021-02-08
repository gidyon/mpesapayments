package c2b

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/gidyon/mpesapayments/pkg/util/timeutil"
	"gorm.io/gorm"
)

func (mpesaAPI *mpesaAPIServer) dailyStatWorker(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Hour)
	defer ticker.Stop()

loop:
	for {
		currTime := time.Now()
		select {
		case <-ctx.Done():
		case <-ticker.C:
			currTime2 := time.Now()

			lockKey := fmt.Sprintf("workerlock:%s", currTime2.UTC().String()[:15])
			existRes, err := mpesaAPI.RedisDB.Exists(ctx, lockKey).Result()
			if err != nil {
				mpesaAPI.Logger.Errorf("faile to check if key exists: %v", err)
				continue
			}

			if existRes == 0 {
				// Key does not exist so we set it
				err = mpesaAPI.RedisDB.Set(ctx, lockKey, "yes", 3*time.Hour).Err()
				if err != nil {
					mpesaAPI.Logger.Errorf("faile to set lock key: %v", err)
					continue
				}
			} else if existRes == 1 {
				// Key does exist so we dont generate the statistics
				continue
			}

			if currTime2.Day() != currTime.Day() {
				// we're in the next day; lets update current statistics for yesterday first
				startTime, err := timeutil.ParseDayStartTime(int32(currTime.Year()), int32(currTime.Month()), int32(currTime.Day()))
				if err != nil {
					mpesaAPI.Logger.Errorf("WORKER: failed to create start timestamp: %v", err)
					goto loop
				}
				endTime := startTime.Add(time.Hour * 24)

				mpesaAPI.generateStatistics(ctx, startTime.Unix(), endTime.Unix())
			} else {
				endTime, err := timeutil.ParseDayEndTime(int32(currTime2.Year()), int32(currTime2.Month()), int32(currTime2.Day()))
				if err != nil {
					mpesaAPI.Logger.Errorf("WORKER: failed to create end timestamp: %v", err)
					goto loop
				}

				mpesaAPI.generateStatistics(ctx, endTime.Unix()-int64(24*60*60), endTime.Unix())
			}
		}
	}
}

type shortCode struct {
	BusinessShortCode string
}
type referenceNumber struct {
	ReferenceNumber string
}

func (mpesaAPI *mpesaAPIServer) generateStatistics(ctx context.Context, startTimestamp, endTimestamp int64) {
	// Get all unique short_code
	shortCodes := make([]*shortCode, 0)
	err := mpesaAPI.SQLDB.Table((&PaymentMpesa{}).TableName()).
		Where("transaction_timestamp BETWEEN ? AND ?", startTimestamp, endTimestamp).
		Distinct("business_short_code").
		Select("business_short_code").
		Scan(&shortCodes).Error
	if err != nil {
		mpesaAPI.Logger.Errorf("WORKER: failed to scan business_short_code to slice: %v", err)
		return
	}

	// Generate report
	for _, shortCode := range shortCodes {
		// Get all unique account_numbers for short_code
		accountNumbers := make([]*referenceNumber, 0)
		err = mpesaAPI.SQLDB.Model(&PaymentMpesa{}).
			Where("transaction_timestamp BETWEEN ? AND ?", startTimestamp, endTimestamp).
			Where("business_short_code = ?", shortCode.BusinessShortCode).
			Distinct("reference_number").
			Select("reference_number").
			Scan(&accountNumbers).Error
		if err != nil {
			mpesaAPI.Logger.Errorf("WORKER: failed to scan reference_number to slice: %v", err)
			continue
		}

		// Get statictics for each account number but first merged
		for _, accountNumber := range accountNumbers {

			db := mpesaAPI.SQLDB.Model(&PaymentMpesa{}).
				Where("transaction_timestamp BETWEEN ? AND ?", startTimestamp, endTimestamp).
				Where("business_short_code = ?", shortCode.BusinessShortCode).
				Where("reference_number = ?", accountNumber.ReferenceNumber)

			var transactions int64

			// Count of transactions
			err = db.Count(&transactions).Error
			if err != nil {
				mpesaAPI.Logger.Errorf(
					"WORKER: failed to count transactions count for day_seconds %v short_code %s account %s : %v",
					startTimestamp, shortCode.BusinessShortCode, accountNumber, err,
				)
				return
			}

			var totalAmount sql.NullFloat64

			// Get total amount
			err = db.Model(&PaymentMpesa{}).Select("sum(amount) as total").Row().Scan(&totalAmount)
			if err != nil {
				mpesaAPI.Logger.Errorf(
					"WORKER: failed to get sum of transactions for day_seconds %v short_code %s account %s : %v",
					startTimestamp, shortCode.BusinessShortCode, accountNumber.ReferenceNumber, err,
				)
				return
			}

			var totalAmountF float32

			if totalAmount.Valid {
				totalAmountF = float32(totalAmount.Float64)
			}

			date := time.Unix(startTimestamp, 0).UTC().String()[:10]
			// Create stat
			statDB := &Stat{
				ShortCode:         shortCode.BusinessShortCode,
				AccountName:       accountNumber.ReferenceNumber,
				Date:              date,
				TotalAmount:       totalAmountF,
				TotalTransactions: int32(transactions),
			}

			statDB2 := &Stat{}

			// Save statistics
			err = mpesaAPI.SQLDB.First(
				statDB2, "short_code = ? AND account_name = ? AND date = ?",
				shortCode.BusinessShortCode, accountNumber.ReferenceNumber, date,
			).Error
			switch {
			case err == nil:
				// Update
				err = mpesaAPI.SQLDB.Where("stat_id = ?", statDB2.StatID).Updates(statDB).Error
				if err != nil {
					mpesaAPI.Logger.Errorf(
						"WORKER: failed to update stats for day_seconds %v short_code %s account %s : %v",
						startTimestamp, shortCode.BusinessShortCode, accountNumber.ReferenceNumber, err,
					)
					return
				}
			case errors.Is(err, gorm.ErrRecordNotFound):
				// Create
				err = mpesaAPI.SQLDB.Create(statDB).Error
				if err != nil {
					mpesaAPI.Logger.Errorf(
						"WORKER: failed to create stats for day_seconds %v short_code %s account %s : %v",
						startTimestamp, shortCode.BusinessShortCode, accountNumber.ReferenceNumber, err,
					)
					return
				}
			default:
				mpesaAPI.Logger.Errorf("WORKER: failed to find stat %v", err)
				return
			}
		}
	}
}
