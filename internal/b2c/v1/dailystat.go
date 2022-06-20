package b2c

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	b2c_model "github.com/gidyon/mpesapayments/internal/b2c"
	"github.com/gidyon/mpesapayments/pkg/utils/timeutil"
	"gorm.io/gorm"
)

func (b2cAPI *b2cAPIServer) dailyDailyStatWorker(ctx context.Context) {
	ticker := time.NewTicker(2*time.Hour + time.Minute)
	defer ticker.Stop()

loop:
	for {
		currTime := time.Now()
		select {
		case <-ctx.Done():
		case <-ticker.C:
			currTime2 := time.Now()

			lockKey := fmt.Sprintf("workerlock:%s", currTime2.UTC().String()[:15])
			existRes, err := b2cAPI.RedisDB.Exists(ctx, lockKey).Result()
			if err != nil {
				b2cAPI.Logger.Errorf("failed to check if key exists: %v", err)
				continue
			}

			if existRes == 0 {
				// Key does not exist so we set it
				err = b2cAPI.RedisDB.Set(ctx, lockKey, "yes", 2*time.Hour).Err()
				if err != nil {
					b2cAPI.Logger.Errorf("failed to set lock key: %v", err)
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
					b2cAPI.Logger.Errorf("WORKER: failed to parse start timestamp: %v", err)
					goto loop
				}
				endTime := startTime.Add(time.Hour * 24)

				b2cAPI.generateDailyStatistics(ctx, startTime, &endTime)
			} else {
				endTime, err := timeutil.ParseDayEndTime(int32(currTime2.Year()), int32(currTime2.Month()), int32(currTime2.Day()))
				if err != nil {
					b2cAPI.Logger.Errorf("WORKER: failed to parse end timestamp: %v", err)
					goto loop
				}

				startTime := time.Unix(endTime.Unix()-int64(24*60*60), 0)

				b2cAPI.generateDailyStatistics(ctx, &startTime, &endTime)
			}
		}
	}
}

type shortCode struct {
	OrgShortCode string
}

func (b2cAPI *b2cAPIServer) generateDailyStatistics(ctx context.Context, startTime, endTime *time.Time) {

	// Get all unique org_short_code
	shortCodes := make([]*shortCode, 0)
	err := b2cAPI.SQLDB.Table((&b2c_model.Payment{}).TableName()).
		Where("transaction_time BETWEEN ? AND ?", startTime, endTime).
		Distinct("org_short_code").
		Select("org_short_code").
		Scan(&shortCodes).Error
	if err != nil {
		b2cAPI.Logger.Errorf("WORKER: failed to scan org_short_code to slice: %v", err)
		return
	}

	// Generate report
	for _, shortCode := range shortCodes {

		db := b2cAPI.SQLDB.Model(&b2c_model.Payment{}).
			Where("transaction_time BETWEEN ? AND ?", startTime, endTime).
			Where("org_short_code = ?", shortCode.OrgShortCode)

		date := startTime.String()[:10]

		var transactions int64

		// Count of total transactions
		err = db.Count(&transactions).Error
		if err != nil {
			b2cAPI.Logger.Errorf(
				"WORKER: failed to count transactions for day %s org_short_code %s: %v",
				date, shortCode.OrgShortCode, err,
			)
			return
		}

		var successfulTransactions int64

		// Count of successful transactions
		err = db.Where("succeeded = ?", true).Count(&successfulTransactions).Error
		if err != nil {
			b2cAPI.Logger.Errorf(
				"WORKER: failed to count successful transactions for day [%s] org_short_code [%s]: %v",
				date, shortCode.OrgShortCode, err,
			)
			return
		}

		var totalAmount sql.NullFloat64

		// Get total amount transacted
		err = db.Model(&b2c_model.Payment{}).Select("sum(amount) as total").Row().Scan(&totalAmount)
		if err != nil {
			b2cAPI.Logger.Errorf(
				"WORKER: failed to get sum of transactions for day [%]s org_short_code [%s]: %v",
				date, shortCode.OrgShortCode, err,
			)
			return
		}

		var totalCharges sql.NullFloat64

		// Get total charges
		err = db.Model(&b2c_model.Payment{}).Select("sum(transaction_charge) as total").Row().Scan(&totalCharges)
		if err != nil {
			b2cAPI.Logger.Errorf(
				"WORKER: failed to get sum of transactions for day %s org_short_code %s: %v",
				date, shortCode.OrgShortCode, err,
			)
			return
		}

		var totalAmountF, totalChargesF float32

		if totalAmount.Valid {
			totalAmountF = float32(totalAmount.Float64)
		}
		if totalCharges.Valid {
			totalChargesF = float32(totalCharges.Float64)
		}

		// Create stat
		statDB := &b2c_model.DailyStat{
			OrgShortCode:           shortCode.OrgShortCode,
			Date:                   date,
			TotalTransactions:      int32(transactions),
			SuccessfulTransactions: int32(successfulTransactions),
			FailedTransactions:     int32(transactions) - int32(successfulTransactions),
			TotalAmountTransacted:  totalAmountF,
			TotalCharges:           totalChargesF,
		}

		statDB2 := &b2c_model.DailyStat{}

		// Save statistics
		err = b2cAPI.SQLDB.First(statDB2, "org_short_code = ? AND date = ?", shortCode.OrgShortCode, date).Error
		switch {
		case err == nil:
			// Update
			err = b2cAPI.SQLDB.Where("id = ?", statDB2.ID).Updates(statDB).Error
			if err != nil {
				b2cAPI.Logger.Errorf(
					"WORKER: failed to update stats for day [%s] org_short_code [%s] : %v", date, shortCode.OrgShortCode, err,
				)
				return
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			// Create
			err = b2cAPI.SQLDB.Create(statDB).Error
			if err != nil {
				b2cAPI.Logger.Errorf(
					"WORKER: failed to create stats for day [%s] org_short_code [%s] : %v", date, shortCode.OrgShortCode, err,
				)
				return
			}
		default:
			b2cAPI.Logger.Errorf("WORKER: failed to find stat %v", err)
			return
		}
	}
}
