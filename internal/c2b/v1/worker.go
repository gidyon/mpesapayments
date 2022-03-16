package c2b

// func (c2bAPI *c2bAPIServer) worker(ctx context.Context, dur time.Duration) {
// 	ticker := time.NewTicker(dur)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-ticker.C:
// 			err := c2bAPI.saveFailedTransactions()
// 			switch {
// 			case err == nil || errors.Is(err, io.EOF):
// 			case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
// 			default:
// 				c2bAPI.Logger.Errorf("error while running worker: %v", err)
// 			}
// 		}
// 	}
// }

// func (c2bAPI *c2bAPIServer) saveFailedTransactions() error {

// 	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
// 	defer cancel()

// 	count := 0

// 	defer func() {
// 		c2bAPI.Logger.Infof("%d failed transactions saved in database", count)
// 	}()

// loop:
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		default:
// 			res, err := c2bAPI.RedisDB.BRPopLPush(
// 				ctx,
// 				c2bAPI.AddPrefix(FailedTxList),
// 				c2bAPI.AddPrefix(failedTxListv2),
// 				5*time.Minute,
// 			).Result()
// 			switch {
// 			case err == nil:
// 			case errors.Is(err, redis.Nil):
// 			default:
// 				if err != nil {
// 					return err
// 				}
// 				if res == "" {
// 					return nil
// 				}
// 			}

// 			// Unmarshal
// 			mpesaTransaction := &c2b.C2BPayment{}
// 			err = proto.Unmarshal([]byte(res), mpesaTransaction)
// 			if err != nil {
// 				c2bAPI.Logger.Errorf("failed to proto unmarshal failed mpesa transaction: %v", err)
// 				goto loop
// 			}

// 			// Validation
// 			err = ValidateC2BPayment(mpesaTransaction)
// 			if err != nil {
// 				c2bAPI.Logger.Errorf("validation failed for mpesa transaction: %v", err)
// 				goto loop
// 			}

// 			mpesaDB, err := C2BPaymentDB(mpesaTransaction)
// 			if err != nil {
// 				c2bAPI.Logger.Errorf("failed to get mpesa database model: %v", err)
// 				goto loop
// 			}

// 			// Save to database
// 			err = c2bAPI.SQLDB.Create(mpesaDB).Error
// 			switch {
// 			case err == nil:
// 				count++
// 			case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
// 				c2bAPI.Logger.Infoln("skipping mpesa transaction since it is available in database")
// 				goto loop
// 			default:
// 				c2bAPI.Logger.Errorf("failed to save mpesa transaction: %v", err)
// 				goto loop
// 			}

// 			c2bAPI.Logger.Infoln("mpesa transaction saved in database")
// 		}
// 	}
// }
