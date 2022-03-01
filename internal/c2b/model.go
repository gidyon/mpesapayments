package c2b

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gidyon/micro/v2/utils/errs"
	"github.com/gidyon/mpesapayments/pkg/api/c2b"
	"gorm.io/gorm"
)

// C2Bs is table for mpesa payments
const C2Bs = "c2b_transactions"

var tablePrefix = ""

// PaymentMpesa contains mpesa transaction details
type PaymentMpesa struct {
	PaymentID         uint      `gorm:"primaryKey;autoIncrement"`
	TransactionType   string    `gorm:"type:varchar(50);not null"`
	TransactionID     string    `gorm:"type:varchar(50);not null;unique"`
	MSISDN            string    `gorm:"index;type:varchar(15);not null"`
	Names             string    `gorm:"type:varchar(50)"`
	ReferenceNumber   string    `gorm:"index;type:varchar(20)"`
	Amount            float32   `gorm:"type:float(10);not null"`
	OrgAccountBalance float32   `gorm:"type:float(10)"`
	BusinessShortCode int32     `gorm:"index;type:varchar(10);not null"`
	TransactionTime   time.Time `gorm:"autoCreateTime"`
	CreateTime        time.Time `gorm:"autoCreateTime"`
	Processed         bool      `gorm:"type:tinyint(1);not null"`
}

// TableName returns the name of the table
func (*PaymentMpesa) TableName() string {
	if tablePrefix != "" {
		return fmt.Sprintf("%s_%s", tablePrefix, C2Bs)
	}
	return C2Bs
}

// C2BPaymentDB converts protobuf mpesa message to C2BMpesa
func C2BPaymentDB(MpesaPB *c2b.C2BPayment) (*PaymentMpesa, error) {
	if MpesaPB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaDB := &PaymentMpesa{
		TransactionID:     MpesaPB.TransactionId,
		TransactionType:   MpesaPB.TransactionType,
		TransactionTime:   time.Unix(MpesaPB.TransactionTimeSeconds, 0),
		MSISDN:            MpesaPB.Msisdn,
		Names:             MpesaPB.Names,
		ReferenceNumber:   MpesaPB.RefNumber,
		Amount:            MpesaPB.Amount,
		OrgAccountBalance: MpesaPB.OrgBalance,
		BusinessShortCode: MpesaPB.BusinessShortCode,
		Processed:         MpesaPB.Processed,
	}

	return mpesaDB, nil
}

// C2BPayment returns the protobuf message of mpesa payment model
func C2BPayment(MpesaDB *PaymentMpesa) (*c2b.C2BPayment, error) {
	if MpesaDB == nil {
		return nil, errs.NilObject("mpesa payment")
	}

	mpesaPB := &c2b.C2BPayment{
		PaymentId:              fmt.Sprint(MpesaDB.PaymentID),
		TransactionType:        MpesaDB.TransactionType,
		TransactionId:          MpesaDB.TransactionID,
		TransactionTimeSeconds: MpesaDB.TransactionTime.Unix(),
		Msisdn:                 MpesaDB.MSISDN,
		Names:                  MpesaDB.Names,
		RefNumber:              MpesaDB.ReferenceNumber,
		Amount:                 MpesaDB.Amount,
		OrgBalance:             MpesaDB.OrgAccountBalance,
		BusinessShortCode:      MpesaDB.BusinessShortCode,
		Processed:              MpesaDB.Processed,
		CreateTimeSeconds:      MpesaDB.CreateTime.Unix(),
	}

	return mpesaPB, nil
}

const statsTable = "daily_stats"

// Stat is model containing transactions statistic
type Stat struct {
	StatID            uint           `gorm:"primaryKey;autoIncrement"`
	ShortCode         string         `gorm:"index;type:varchar(20);not null"`
	AccountName       string         `gorm:"index;type:varchar(20);not null"`
	Date              string         `gorm:"index;type:varchar(10);not null"`
	TotalTransactions int32          `gorm:"type:int(10);not null"`
	TotalAmount       float32        `gorm:"type:float(12);not null"`
	CreatedAt         time.Time      `gorm:"autoCreateTime"`
	UpdatedAt         time.Time      `gorm:"autoCreateTime"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

// TableName ...
func (*Stat) TableName() string {
	// Get table prefix
	if tablePrefix != "" {
		return fmt.Sprintf("%s_%s", tablePrefix, statsTable)
	}
	return statsTable
}

// GetStatDB gets mpesa statistics model from protobuf message
func GetStatDB(statPB *c2b.Stat) (*Stat, error) {
	return &Stat{
		ShortCode:         statPB.ShortCode,
		AccountName:       statPB.AccountName,
		Date:              statPB.ShortCode,
		TotalTransactions: statPB.TotalTransactions,
		TotalAmount:       statPB.TotalAmount,
	}, nil
}

// GetStatPB gets mpesa statistics protobuf from model
func GetStatPB(statDB *Stat) (*c2b.Stat, error) {
	return &c2b.Stat{
		StatId:            fmt.Sprint(statDB.StatID),
		Date:              statDB.ShortCode,
		ShortCode:         statDB.ShortCode,
		AccountName:       statDB.AccountName,
		TotalTransactions: statDB.TotalTransactions,
		TotalAmount:       statDB.TotalAmount,
		CreateDateSeconds: statDB.CreatedAt.Unix(),
		UpdateTimeSeconds: statDB.UpdatedAt.Unix(),
	}, nil
}

// Scopes is model user scopes
type Scopes struct {
	UserID    string         `gorm:"primaryKey;type:varchar(15)"`
	Scopes    []byte         `gorm:"type:json"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoCreateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName ...
func (*Scopes) TableName() string {
	return "scopes"
}

// QueueBulk contains exported data for bulk
type QueueBulk struct {
	// ID           uint      `gorm:"primaryKey;autoIncrement"`
	// Originator   string    `gorm:"index;type:varchar(25);not null;default:''"`
	// OriginatorID string    `gorm:"index;type:varchar(20);not null;default:''"`
	// Destination  string    `gorm:"index;type:varchar(15);not null;default:''"`
	// Message      string    `gorm:"index;type:varchar(25);not null;default:''"`
	// Keyword      string    `gorm:"index;type:varchar(25)"`
	// SMSCID       string    `gorm:"index;type:varchar(25)"`
	// Processed    bool      `gorm:"type:tinyint(1);not null;default:0"`
	// CreateTime   time.Time `gorm:"autoCreateTime"`
	// UpdatedAt    time.Time `gorm:"autoCreateTime"`
	// DeletedAt    gorm.DeletedAt

	RecordID         uint      `gorm:"primaryKey;autoIncrement;column:RecordID"`
	Originator       string    `gorm:"index;type:varchar(25);not null;default:'20453';column:Originator"`
	Destination      string    `gorm:"index;type:varchar(15);not null;column:Destination"`
	Message          string    `gorm:"type:text(500);column:Message"`
	MessageTimeStamp time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;column:MessageTimeStamp"`
	MessageDirection string    `gorm:"type:varchar(5);column:MessageTimeStamp"`
	SMSCID           string    `gorm:"index;type:varchar(20);column:SMSCID"`
	Command          bool      `gorm:"type:varchar(20);column:command"`
}

// TableName ...
func (*QueueBulk) TableName() string {
	// Get table prefix
	table := os.Getenv("BLAST_QUEUE_TABLE")
	if table != "" {
		return table
	}
	return "dbQueue_Bulk"
}

// Originator, Destination, Message, MessageTimeStamp, MessageDirection, SMSCID, command

// BlastReport is model for export statistics
type BlastReport struct {
	ID            uint      `gorm:"primaryKey;autoIncrement"`
	Originator    string    `gorm:"index;type:varchar(25);not null"`
	Initiator     string    `gorm:"index;type:varchar(50);not null"`
	Source        string    `gorm:"index;type:varchar(20);not null"`
	BlastFile     string    `gorm:"index;type:varchar(50)"`
	Message       string    `gorm:"type:text(500);not null"`
	TotalExported int32     `gorm:"index;type:int(10);not null;default:0"`
	ExportFilter  []byte    `gorm:"type:json;"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

// TableName ...
func (*BlastReport) TableName() string {
	return "blast_reports"
}

// GetExportDB gets export db model from protobuf message
func GetExportDB(exportPB *c2b.BlastReport) (*BlastReport, error) {
	exportDB := &BlastReport{
		Originator:    exportPB.Originator,
		Initiator:     exportPB.Initiator,
		Source:        exportPB.Source,
		BlastFile:     fmt.Sprintf("%s:%s", exportPB.BlastFileId, exportPB.BlastFile),
		Message:       exportPB.Comment,
		TotalExported: exportPB.TotalExported,
	}
	if exportDB.ExportFilter != nil {
		bs, err := json.Marshal(exportPB.ExportFilter)
		if err != nil {
			return nil, errs.FromJSONMarshal(err, "export filter")
		}
		exportDB.ExportFilter = bs
	}
	return exportDB, nil
}

// GetExportPB gets protobuf message from db model
func GetExportPB(exportDB *BlastReport) (*c2b.BlastReport, error) {
	var (
		files       = strings.Split(exportDB.BlastFile, ":")
		blastFile   string
		blastFileID string
	)
	if len(files) == 2 {
		blastFile = files[1]
		blastFileID = files[0]
	}
	exportPB := &c2b.BlastReport{
		ReportId:          fmt.Sprint(exportDB.ID),
		Comment:           exportDB.Message,
		Initiator:         exportDB.Initiator,
		Originator:        exportDB.Originator,
		Source:            exportDB.Source,
		BlastFile:         blastFile,
		BlastFileId:       blastFileID,
		TotalExported:     exportDB.TotalExported,
		CreateDateSeconds: exportDB.CreatedAt.Unix(),
	}
	if len(exportDB.ExportFilter) > 0 {
		err := json.Unmarshal(exportDB.ExportFilter, exportPB.ExportFilter)
		if err != nil {
			return nil, errs.FromJSONUnMarshal(err, "export filter")
		}
	}
	return exportPB, nil
}

// BlastFile is model for uplaoded file
type BlastFile struct {
	ID            uint      `gorm:"primaryKey;autoIncrement"`
	ReferenceName string    `gorm:"index;type:varchar(50);not null;unique"`
	FileName      string    `gorm:"index;type:varchar(50);not null"`
	UploaderNames string    `gorm:"index;type:varchar(50);not null"`
	TotalMsisdn   int32     `gorm:"index;type:int(10);not null"`
	UploadedAt    time.Time `gorm:"autoCreateTime"`
}

// TableName ...
func (*BlastFile) TableName() string {
	return "blast_files"
}

// GetBlastFileDB gets the blast file model from protobuf message
func GetBlastFileDB(blastFilePB *c2b.BlastFile) (*BlastFile, error) {
	blastFileDB := &BlastFile{
		ReferenceName: blastFilePB.ReferenceName,
		FileName:      blastFilePB.FileName,
		UploaderNames: blastFilePB.UploaderNames,
		TotalMsisdn:   blastFilePB.TotalMsisdn,
	}
	return blastFileDB, nil
}

// GetBlastFilePB gets blast file protobuf message from blast file model
func GetBlastFilePB(blastFileDB *BlastFile) (*c2b.BlastFile, error) {
	blastFilePB := &c2b.BlastFile{
		FileId:            fmt.Sprint(blastFileDB.ID),
		ReferenceName:     blastFileDB.ReferenceName,
		FileName:          blastFileDB.FileName,
		UploaderNames:     blastFileDB.UploaderNames,
		UploadTimeSeconds: blastFileDB.UploadedAt.Unix(),
		TotalMsisdn:       blastFileDB.TotalMsisdn,
	}
	return blastFilePB, nil
}

// UploadedFileData contains data contained in the uploaded file
type UploadedFileData struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Msisdn    string    `gorm:"index;type:varchar(15);not null"`
	FileID    string    `gorm:"index;type:varchar(10);not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName ...
func (*UploadedFileData) TableName() string {
	return "uploaded_msisdns"
}
