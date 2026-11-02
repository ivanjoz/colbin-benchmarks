// Wrappers are one measured case each: a table on its own, so a case is
// one table the way publicbi measures one workbook table. Generated: see
// /tmp/opencode/tpcds/gen_types.py.
package tpcds

// CallCenterTable is the TPC-DS CallCenter table as one case.
type CallCenterTable struct {
	Rows []CallCenter `cb:"1"`
}

// CatalogPageTable is the TPC-DS CatalogPage table as one case.
type CatalogPageTable struct {
	Rows []CatalogPage `cb:"1"`
}

// CatalogReturnsTable is the TPC-DS CatalogReturns table as one case.
type CatalogReturnsTable struct {
	Rows []CatalogReturns `cb:"1"`
}

// CatalogSalesTable is the TPC-DS CatalogSales table as one case.
type CatalogSalesTable struct {
	Rows []CatalogSales `cb:"1"`
}

// CustomerTable is the TPC-DS Customer table as one case.
type CustomerTable struct {
	Rows []Customer `cb:"1"`
}

// CustomerAddressTable is the TPC-DS CustomerAddress table as one case.
type CustomerAddressTable struct {
	Rows []CustomerAddress `cb:"1"`
}

// CustomerDemographicsTable is the TPC-DS CustomerDemographics table as one case.
type CustomerDemographicsTable struct {
	Rows []CustomerDemographics `cb:"1"`
}

// DateDimTable is the TPC-DS DateDim table as one case.
type DateDimTable struct {
	Rows []DateDim `cb:"1"`
}

// HouseholdDemographicsTable is the TPC-DS HouseholdDemographics table as one case.
type HouseholdDemographicsTable struct {
	Rows []HouseholdDemographics `cb:"1"`
}

// IncomeBandTable is the TPC-DS IncomeBand table as one case.
type IncomeBandTable struct {
	Rows []IncomeBand `cb:"1"`
}

// InventoryTable is the TPC-DS Inventory table as one case.
type InventoryTable struct {
	Rows []Inventory `cb:"1"`
}

// ItemTable is the TPC-DS Item table as one case.
type ItemTable struct {
	Rows []Item `cb:"1"`
}

// PromotionTable is the TPC-DS Promotion table as one case.
type PromotionTable struct {
	Rows []Promotion `cb:"1"`
}

// ReasonTable is the TPC-DS Reason table as one case.
type ReasonTable struct {
	Rows []Reason `cb:"1"`
}

// ShipModeTable is the TPC-DS ShipMode table as one case.
type ShipModeTable struct {
	Rows []ShipMode `cb:"1"`
}

// StoreTable is the TPC-DS Store table as one case.
type StoreTable struct {
	Rows []Store `cb:"1"`
}

// StoreReturnsTable is the TPC-DS StoreReturns table as one case.
type StoreReturnsTable struct {
	Rows []StoreReturns `cb:"1"`
}

// StoreSalesTable is the TPC-DS StoreSales table as one case.
type StoreSalesTable struct {
	Rows []StoreSales `cb:"1"`
}

// TimeDimTable is the TPC-DS TimeDim table as one case.
type TimeDimTable struct {
	Rows []TimeDim `cb:"1"`
}

// WarehouseTable is the TPC-DS Warehouse table as one case.
type WarehouseTable struct {
	Rows []Warehouse `cb:"1"`
}

// WebPageTable is the TPC-DS WebPage table as one case.
type WebPageTable struct {
	Rows []WebPage `cb:"1"`
}

// WebReturnsTable is the TPC-DS WebReturns table as one case.
type WebReturnsTable struct {
	Rows []WebReturns `cb:"1"`
}

// WebSalesTable is the TPC-DS WebSales table as one case.
type WebSalesTable struct {
	Rows []WebSales `cb:"1"`
}

// WebSiteTable is the TPC-DS WebSite table as one case.
type WebSiteTable struct {
	Rows []WebSite `cb:"1"`
}
