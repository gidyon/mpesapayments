package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gidyon/micro/pkg/conn"
	"github.com/gidyon/micro/utils/errs"
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
	errs.Panic(err)
	_ = conn

	// Auto migrate
	// errs.Panic(conn.Migrator().AutoMigrate(&bulkQueue{}))

	// fmt.Println("done migrations")

	fmt.Printf("filename is %s\n", *fileName)

	// Read files
	f, err := os.Open(*fileName)
	errs.Panic(err)

	defer f.Close()

	s := bufio.NewScanner(f)

	message := "Kuna Ksh 10,000 ya kushindaniwa na watu 50 katika draw inayofanywa GUKENA FM 8pm-12pm\n\nUtakuwa mmoja wao?\n\nTuma Ksh 100\nPaybill: 4062507\nAccount: G\n\nStop*456*9*5#"

	const bulkSize = 1000

	bulks := make([]*bulkQueue, 0, bulkSize)

	count := 0

	for s.Scan() {
		if count < 70000 {
			count++
			continue
		}
		if count > 100000 {
			break
		}
		phone := strings.TrimSpace(s.Text())
		if phone == "" {
			continue
		}
		phone2, err := strconv.Atoi(phone)
		if err != nil {
			fmt.Println("failed to parse phone: ", phone)
			continue
		}
		bulks = append(bulks, &bulkQueue{
			Originator:       "20453",
			Destination:      fmt.Sprint(phone2),
			Message:          message,
			SMSCID:           "SAFARICOM_KE",
			MessageDirection: "OUT",
		})
		if len(bulks) >= bulkSize {
			fmt.Printf("inserted %d records\n", len(bulks))
			err = conn.CreateInBatches(bulks, bulkSize+1).Error
			errs.Panic(err)
			bulks = bulks[0:0]
		}
		count++
	}

	fmt.Printf("blasted %d records\n", count)
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
