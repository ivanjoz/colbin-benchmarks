// Wrappers are one measured case each: a table on its own, the way tpcds and
// publicbi measure one table per case. Nested and Normalized stay as the
// whole-dataset cases.
package tpch

// OrdersTable is the TPC-H orders table as one case.
type OrdersTable struct {
	Rows []OrderRow `cb:"1"`
}

// LineItemTable is the TPC-H lineitem table as one case.
type LineItemTable struct {
	Rows []LineItemRow `cb:"1"`
}

// CustomerTable is the TPC-H customer table as one case.
type CustomerTable struct {
	Rows []Customer `cb:"1"`
}

// PartTable is the TPC-H part table as one case.
type PartTable struct {
	Rows []Part `cb:"1"`
}

// PartSuppTable is the TPC-H partsupp table as one case.
type PartSuppTable struct {
	Rows []PartSupp `cb:"1"`
}

// SupplierTable is the TPC-H supplier table as one case.
type SupplierTable struct {
	Rows []Supplier `cb:"1"`
}

// NationTable is the TPC-H nation table as one case.
type NationTable struct {
	Rows []Nation `cb:"1"`
}

// RegionTable is the TPC-H region table as one case.
type RegionTable struct {
	Rows []Region `cb:"1"`
}
