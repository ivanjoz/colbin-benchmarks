// Package tpch is TPC-H, as dbgen writes it, in two shapes:
//
//	Nested      []Order, each holding its own []LineItem — the shape a service
//	            answering "orders with their lines" returns; only orders and
//	            lineitem have that parent and child
//	Normalized  all eight tables side by side, as TPC-H itself defines them:
//	            orders, lineitem, customer, part, partsupp, supplier, nation
//	            and region
//
// Both shapes were fixed before anything was measured, and both are reported.
//
// Every format is handed the same Go values. The type choices are the ones a
// Go programmer would make for these columns, not ones tuned to any format:
//
//   - DECIMAL(15,2) columns are int64 cents, which is exact. A float64 would
//     be lossy for money and is not what TPC-H's DECIMAL means.
//   - DATE columns are int32 days since 1970-01-01.
//   - Keys and other INTEGER columns are int32: TPC-H keys at any scale factor
//     this benchmark runs fit, and int32 is the narrowest type that holds them,
//     which is the fair choice for the fixed-width formats.
//   - CHAR and VARCHAR columns are strings.
//   - c_acctbal and s_acctbal go negative, so they carry pb:"sint".
package tpch

// Nested is the orders table with each order's lines inside it.
type Nested struct {
	Orders []Order `cb:"1"`
}

// Order is one row of orders with its lineitem rows. A line's l_orderkey is
// its order's key, so it is not repeated in the line.
type Order struct {
	OrderKey      int32      `cb:"1"`
	CustKey       int32      `cb:"2"`
	OrderStatus   string     `cb:"3"`
	TotalPrice    int64      `cb:"4"`
	OrderDate     int32      `cb:"5"`
	OrderPriority string     `cb:"6"`
	Clerk         string     `cb:"7"`
	ShipPriority  int32      `cb:"8"`
	Comment       string     `cb:"9"`
	Lines         []LineItem `cb:"10"`
}

// LineItem is one row of lineitem, without l_orderkey.
type LineItem struct {
	PartKey       int32  `cb:"1"`
	SuppKey       int32  `cb:"2"`
	LineNumber    int32  `cb:"3"`
	Quantity      int32  `cb:"4"`
	ExtendedPrice int64  `cb:"5"`
	Discount      int32  `cb:"6"` // hundredths
	Tax           int32  `cb:"7"` // hundredths
	ReturnFlag    string `cb:"8"`
	LineStatus    string `cb:"9"`
	ShipDate      int32  `cb:"10"`
	CommitDate    int32  `cb:"11"`
	ReceiptDate   int32  `cb:"12"`
	ShipInstruct  string `cb:"13"`
	ShipMode      string `cb:"14"`
	Comment       string `cb:"15"`
}

// Normalized is the eight tables as TPC-H defines them.
type Normalized struct {
	Orders    []OrderRow    `cb:"1"`
	Lines     []LineItemRow `cb:"2"`
	Customers []Customer    `cb:"3"`
	Parts     []Part        `cb:"4"`
	PartSupps []PartSupp    `cb:"5"`
	Suppliers []Supplier    `cb:"6"`
	Nations   []Nation      `cb:"7"`
	Regions   []Region      `cb:"8"`
}

// OrderRow is one row of orders.
type OrderRow struct {
	OrderKey      int32  `cb:"1"`
	CustKey       int32  `cb:"2"`
	OrderStatus   string `cb:"3"`
	TotalPrice    int64  `cb:"4"`
	OrderDate     int32  `cb:"5"`
	OrderPriority string `cb:"6"`
	Clerk         string `cb:"7"`
	ShipPriority  int32  `cb:"8"`
	Comment       string `cb:"9"`
}

// LineItemRow is one row of lineitem.
type LineItemRow struct {
	OrderKey      int32  `cb:"1"`
	PartKey       int32  `cb:"2"`
	SuppKey       int32  `cb:"3"`
	LineNumber    int32  `cb:"4"`
	Quantity      int32  `cb:"5"`
	ExtendedPrice int64  `cb:"6"`
	Discount      int32  `cb:"7"`
	Tax           int32  `cb:"8"`
	ReturnFlag    string `cb:"9"`
	LineStatus    string `cb:"10"`
	ShipDate      int32  `cb:"11"`
	CommitDate    int32  `cb:"12"`
	ReceiptDate   int32  `cb:"13"`
	ShipInstruct  string `cb:"14"`
	ShipMode      string `cb:"15"`
	Comment       string `cb:"16"`
}

// Customer is one row of customer.
type Customer struct {
	CustKey    int32  `cb:"1"`
	Name       string `cb:"2"`
	Address    string `cb:"3"`
	NationKey  int32  `cb:"4"`
	Phone      string `cb:"5"`
	AcctBal    int64  `cb:"6" pb:"sint"`
	MktSegment string `cb:"7"`
	Comment    string `cb:"8"`
}

// Part is one row of part.
type Part struct {
	PartKey     int32  `cb:"1"`
	Name        string `cb:"2"`
	Mfgr        string `cb:"3"`
	Brand       string `cb:"4"`
	Type        string `cb:"5"`
	Size        int32  `cb:"6"`
	Container   string `cb:"7"`
	RetailPrice int64  `cb:"8"`
	Comment     string `cb:"9"`
}

// PartSupp is one row of partsupp.
type PartSupp struct {
	PartKey    int32  `cb:"1"`
	SuppKey    int32  `cb:"2"`
	AvailQty   int32  `cb:"3"`
	SupplyCost int64  `cb:"4"`
	Comment    string `cb:"5"`
}

// Supplier is one row of supplier.
type Supplier struct {
	SuppKey   int32  `cb:"1"`
	Name      string `cb:"2"`
	Address   string `cb:"3"`
	NationKey int32  `cb:"4"`
	Phone     string `cb:"5"`
	AcctBal   int64  `cb:"6" pb:"sint"`
	Comment   string `cb:"7"`
}

// Nation is one row of nation.
type Nation struct {
	NationKey int32  `cb:"1"`
	Name      string `cb:"2"`
	RegionKey int32  `cb:"3"`
	Comment   string `cb:"4"`
}

// Region is one row of region.
type Region struct {
	RegionKey int32  `cb:"1"`
	Name      string `cb:"2"`
	Comment   string `cb:"3"`
}
