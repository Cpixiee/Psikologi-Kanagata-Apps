package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

// Product: master data produk
type Product struct {
	Id        int       `orm:"auto;pk" json:"id"`
	Name      string    `orm:"size(255);unique" json:"name"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (p *Product) TableName() string { return "products" }

// ProductMeasurementBatch: 1 sesi pengukuran (punya No Machine / Batch Number / Start/Finish Date)
type ProductMeasurementBatch struct {
	Id          int        `orm:"auto;pk" json:"id"`
	Product     *Product   `orm:"rel(fk);null" json:"product,omitempty"`
	NoMachine   string     `orm:"column(no_machine);size(100)" json:"no_machine"`
	BatchNumber string     `orm:"column(batch_number);size(100)" json:"batch_number"`
	StartDate   time.Time  `orm:"column(start_date);type(datetime)" json:"start_date"`
	FinishDate  *time.Time `orm:"column(finish_date);null;type(datetime)" json:"finish_date,omitempty"`
	Status      string     `orm:"size(20)" json:"status"`
	CreatedBy   *int       `orm:"column(created_by);null" json:"created_by,omitempty"`
	CreatedAt   time.Time  `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (b *ProductMeasurementBatch) TableName() string { return "product_measurement_batches" }

// ProductMeasurement: baris measurement item (MEASUREMENT ITEM / TYPE / SAMPLE INDEX / RESULT)
type ProductMeasurement struct {
	Id              int                     `orm:"auto;pk" json:"id"`
	Batch           *ProductMeasurementBatch `orm:"rel(fk)" json:"batch"`
	MeasurementItem string                  `orm:"column(measurement_item);size(255)" json:"measurement_item"`
	Type            string                  `orm:"size(50)" json:"type"`
	SampleIndex     *int                    `orm:"column(sample_index);null" json:"sample_index,omitempty"`
	Result          string                  `orm:"size(255)" json:"result"`
	CreatedAt       time.Time               `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (m *ProductMeasurement) TableName() string { return "product_measurements" }

func init() {
	orm.RegisterModel(
		new(Product),
		new(ProductMeasurementBatch),
		new(ProductMeasurement),
	)
}

