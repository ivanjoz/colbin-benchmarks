// Package synthetic is colbin's own generated corpus (corpus.Generate, seed
// 20260101), the dataset the earlier benchmarks were built on, put through the
// same matrix as the public datasets so the two can be read side by side.
//
// It is the author's data, shaped by the author, and the paper says so: it is
// here as the baseline the public datasets are checked against, not as
// evidence on its own. The pinned bytes are corpusdata/data/MANIFEST.sha256.
//
// The types are corpus's, declared again here field for field and tag for
// tag, because cmd/gen generates from the types of the package it is given.
// load.go converts between the two without copying, and its test checks the
// declarations still agree.
package synthetic

// The single messages bench/ has always measured: one sensor reading, one
// order with three lines, one string-heavy product.

type Reading struct {
	SensorID  uint64  `cb:"2"`
	Timestamp int64   `cb:"3"`
	Value     float64 `cb:"4"`
	Unit      string  `cb:"5"`
	Samples   []int32 `cb:"6"`
	Valid     bool    `cb:"7"`
}

type Order struct {
	OrderID uint64      `cb:"2"`
	Status  string      `cb:"6"`
	Lines   []OrderLine `cb:"4"`
	Created int64       `cb:"7"`
}

type OrderLine struct {
	Discounts []int32 `cb:"2"`
	Quantity  uint32  `cb:"3"`
	Price     int64   `cb:"4"`
}

type Product struct {
	ID         uint64   `cb:"2"`
	SKU        string   `cb:"3"`
	Name       string   `cb:"4"`
	PriceCents int64    `cb:"5"`
	Stock      uint32   `cb:"6"`
	Categories []string `cb:"7"`
}

// The corpus tables.

type User struct {
	ID        uint32 `cb:"2"`
	Name      string `cb:"3"`
	Email     string `cb:"4"`
	Country   string `cb:"5"`
	Age       uint8  `cb:"6"`
	Active    bool   `cb:"7"`
	CreatedAt int64  `cb:"8"`
}

type CatalogProduct struct {
	ID         uint32   `cb:"2"`
	SKU        string   `cb:"3"`
	Name       string   `cb:"4"`
	PriceCents int64    `cb:"5"`
	Stock      uint32   `cb:"6"`
	CategoryID uint16   `cb:"7"`
	Tags       []string `cb:"8"`
}

type Category struct {
	ID       uint16 `cb:"2"`
	Name     string `cb:"3"`
	ParentID uint16 `cb:"4"`
}

type Store struct {
	ID   uint16  `cb:"2"`
	Code string  `cb:"3"`
	City string  `cb:"4"`
	Lat  float64 `cb:"5"`
	Lon  float64 `cb:"6"`
}

type Sale struct {
	ID            uint64     `cb:"2"`
	UserID        uint32     `cb:"3"`
	StoreID       uint16     `cb:"4"`
	CreatedAt     int64      `cb:"5"`
	SubtotalCents int64      `cb:"6"`
	TaxCents      int64      `cb:"7"`
	TotalCents    int64      `cb:"8"`
	PaidCents     int64      `cb:"9"`
	Detail        []SaleLine `cb:"10"`
}

type SaleLine struct {
	ProductID  uint32 `cb:"2"`
	Quantity   uint32 `cb:"3"`
	UnitCents  int64  `cb:"4"`
	DiscountBP uint16 `cb:"5"`
	TaxBP      uint16 `cb:"6"`
	TotalCents int64  `cb:"7"`
}

type Event struct {
	At      int64             `cb:"2"`
	Level   string            `cb:"3"`
	Actor   string            `cb:"4"`
	Message string            `cb:"5"`
	Frames  []string          `cb:"6"`
	Fields  map[string]string `cb:"7"`
}

type Metric struct {
	SeriesID uint32 `cb:"2"`
	At       int64  `cb:"3"`
	Value    int64  `cb:"4"`
}

// The roots.

// Metrics is a page of the metrics table: three integers a row.
type Metrics struct {
	Rows []Metric `cb:"1"`
}

// Sales is a page of the sales table, each sale with its lines.
type Sales struct {
	Rows []Sale `cb:"1"`
}

// Events is the events table, whose rows hold a map.
type Events struct {
	Rows []Event `cb:"1"`
}

// Dataset is every table as one document.
type Dataset struct {
	Users      []User           `cb:"1"`
	Products   []CatalogProduct `cb:"2"`
	Categories []Category       `cb:"3"`
	Stores     []Store          `cb:"4"`
	Sales      []Sale           `cb:"5"`
	Metrics    []Metric         `cb:"6"`
	Events     []Event          `cb:"7"`
}
