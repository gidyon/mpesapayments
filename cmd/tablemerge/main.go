package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gidyon/micro/pkg/conn"
	"github.com/gidyon/micro/utils/errs"
	c2b "github.com/gidyon/mpesapayments/internal/c2b/v1"
)

var (
	fileName = flag.String("filename", "tables.txt", "File containing all the tables")
)

func main() {
	flag.Parse()

	// Open sql connection
	conn, err := conn.OpenGormConn(&conn.DBOptions{
		Dialect:  "msql",
		Address:  "ec2-13-58-199-243.us-east-2.compute.amazonaws.com",
		User:     "onfon",
		Password: "c#7z(9T*s,Z?5SP~)`Jup&E?+LN8NaVB",
		Schema:   "mawingu-portal",
		ConnPool: &conn.DBConnPoolOptions{},
	})

	fmt.Println("connected to database")

	// Loop over tables
	f, err := os.Open(*fileName)
	errs.Panic(err)

	defer f.Close()

	s := bufio.NewScanner(f)

	// Get unique mpesa payments
	var (
		next     = true
		pageSize = 1000
	)

	// Dest table
	dstTable := (&c2b.PaymentMpesa{}).TableName()

	for s.Scan() {
		tableName := s.Text()
		fmt.Println(tableName)

		var (
			c2b1           = make([]*c2b.PaymentMpesa, 0, pageSize+1)
			c2bs           = make([]*paymentMpesa, 0, pageSize+1)
			paymentID uint = 0
		)

		fmt.Printf("getting records for table: %v\n", tableName)

		// Get all records from table
		for next {
			db := conn.Table(tableName).Limit(int(pageSize + 1)).Order("payment_id DESC")

			if paymentID > 0 {
				db = db.Where("payment_id < ?", paymentID)
			}
			err = db.Find(&c2bs).Error
			errs.Panic(err)

			if len(c2bs) <= pageSize {
				next = false
			}

			fmt.Printf("gotten %d records\n", len(c2bs))

			for _, c2bDB := range c2bs {
				// insert data
				c2bDBV2 := &c2b.PaymentMpesa{
					TransactionType:   c2bDB.TransactionType,
					TransactionID:     c2bDB.TransactionID,
					MSISDN:            c2bDB.MSISDN,
					Names:             c2bDB.Names,
					ReferenceNumber:   c2bDB.ReferenceNumber,
					Amount:            c2bDB.Amount,
					OrgAccountBalance: c2bDB.OrgAccountBalance,
					BusinessShortCode: c2bDB.BusinessShortCode,
					TransactionTime:   time.Unix(c2bDB.TransactionTimestamp, 0),
					CreateTime:        time.Unix(c2bDB.CreateTimesatamp, 0),
					Processed:         c2bDB.Processed,
				}

				c2b1 = append(c2b1, c2bDBV2)
				paymentID = c2bDB.PaymentID
			}

			// bulk insert
			err := conn.Table(dstTable).CreateInBatches(c2b1, pageSize+1).Error
			switch {
			case err == nil:
				fmt.Printf("created %d records\n", len(c2b1))
			case strings.Contains(strings.ToLower(err.Error()), "duplicate"):
				fmt.Println("duplicates skipped")
			default:
				errs.Panic(err)
			}
		}

		next = true

		fmt.Printf("bulk inserted all records in %s into %s table\n\n", tableName, dstTable)
	}

	fmt.Println("done")
}

type bulkQueue struct {
	RecordID         uint      `gorm:"primaryKey;autoIncrement;column:RecordID"`
	Originator       string    `gorm:"index;type:varchar(25);not null;default:'20453';column:Originator"`
	Destination      string    `gorm:"index;type:varchar(15);not null;column:Destination"`
	Message          string    `gorm:"type:text(500);column:Message"`
	MessageTimeStamp time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;column:MessageTimeStamp"`
	MessageDirection string    `gorm:"type:varchar(5);column:MessageTimeStamp"`
	SMSCID           string    `gorm:"index;type:varchar(20);column:SMSCID"`
	Command          bool      `gorm:"type:varchar(20);column:command"`
}

func (*bulkQueue) TableName() string {
	return "dbQueue_Bulk"
}

type paymentMpesa struct {
	PaymentID            uint    `gorm:"primaryKey;autoIncrement"`
	TransactionType      string  `gorm:"type:varchar(50);not null"`
	TransactionID        string  `gorm:"type:varchar(50);not null;unique"`
	MSISDN               string  `gorm:"index;type:varchar(15);not null"`
	Names                string  `gorm:"type:varchar(50)"`
	ReferenceNumber      string  `gorm:"index;type:varchar(20)"`
	Amount               float32 `gorm:"type:float(10);not null"`
	OrgAccountBalance    float32 `gorm:"type:float(10)"`
	BusinessShortCode    int32   `gorm:"index;type:varchar(10);not null"`
	TransactionTimestamp int64   `gorm:"type:autoCreateTime"`
	CreateTimesatamp     int64   `gorm:"autoCreateTime"`
	Processed            bool    `gorm:"type:tinyint(1);not null"`
}
