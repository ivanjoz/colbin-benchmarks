package tpch

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Scales are the scale factors fetch.sh generates, as dbgen spells them.
var Scales = []string{"0.1", "1"}

// Dir is where fetch.sh writes scale factor sf under the data directory.
func Dir(data, sf string) string { return filepath.Join(data, "tpch", "sf"+sf) }

// LoadNormalized reads orders.tbl and lineitem.tbl as dbgen wrote them.
func LoadNormalized(data, sf string) (*Normalized, error) {
	dir := Dir(data, sf)
	var out Normalized
	err := readTable(filepath.Join(dir, "orders.tbl"), 9, func(f []string) error {
		var row OrderRow
		var err error
		row.OrderKey, err = parseInt32(f[0], err)
		row.CustKey, err = parseInt32(f[1], err)
		row.OrderStatus = f[2]
		row.TotalPrice, err = parseCents(f[3], err)
		row.OrderDate, err = parseDate(f[4], err)
		row.OrderPriority = f[5]
		row.Clerk = f[6]
		row.ShipPriority, err = parseInt32(f[7], err)
		row.Comment = f[8]
		out.Orders = append(out.Orders, row)
		return err
	})
	if err != nil {
		return nil, err
	}
	err = readTable(filepath.Join(dir, "lineitem.tbl"), 16, func(f []string) error {
		var row LineItemRow
		var err error
		row.OrderKey, err = parseInt32(f[0], err)
		row.PartKey, err = parseInt32(f[1], err)
		row.SuppKey, err = parseInt32(f[2], err)
		row.LineNumber, err = parseInt32(f[3], err)
		// l_quantity is DECIMAL(15,2) in the spec, always whole, and this dbgen
		// writes it as a bare integer.
		row.Quantity, err = parseInt32(strings.TrimSuffix(f[4], ".00"), err)
		row.ExtendedPrice, err = parseCents(f[5], err)
		discount, err := parseCents(f[6], err)
		row.Discount = int32(discount)
		tax, err := parseCents(f[7], err)
		row.Tax = int32(tax)
		row.ReturnFlag = f[8]
		row.LineStatus = f[9]
		row.ShipDate, err = parseDate(f[10], err)
		row.CommitDate, err = parseDate(f[11], err)
		row.ReceiptDate, err = parseDate(f[12], err)
		row.ShipInstruct = f[13]
		row.ShipMode = f[14]
		row.Comment = f[15]
		out.Lines = append(out.Lines, row)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// LoadNested reads the same files and nests each order's lines inside it.
// dbgen writes lineitem in order-key order, so the lines of one order are
// adjacent; a line out of that order is an error, not something to sort.
func LoadNested(data, sf string) (*Nested, error) {
	flat, err := LoadNormalized(data, sf)
	if err != nil {
		return nil, err
	}
	return Nest(flat)
}

// Nest turns the normalized shape into the nested one.
func Nest(flat *Normalized) (*Nested, error) {
	out := &Nested{Orders: make([]Order, len(flat.Orders))}
	line := 0
	for index, row := range flat.Orders {
		order := Order{
			OrderKey: row.OrderKey, CustKey: row.CustKey, OrderStatus: row.OrderStatus,
			TotalPrice: row.TotalPrice, OrderDate: row.OrderDate,
			OrderPriority: row.OrderPriority, Clerk: row.Clerk,
			ShipPriority: row.ShipPriority, Comment: row.Comment,
		}
		start := line
		for line < len(flat.Lines) && flat.Lines[line].OrderKey == row.OrderKey {
			line++
		}
		order.Lines = make([]LineItem, 0, line-start)
		for _, l := range flat.Lines[start:line] {
			order.Lines = append(order.Lines, LineItem{
				PartKey: l.PartKey, SuppKey: l.SuppKey, LineNumber: l.LineNumber,
				Quantity: l.Quantity, ExtendedPrice: l.ExtendedPrice,
				Discount: l.Discount, Tax: l.Tax, ReturnFlag: l.ReturnFlag,
				LineStatus: l.LineStatus, ShipDate: l.ShipDate,
				CommitDate: l.CommitDate, ReceiptDate: l.ReceiptDate,
				ShipInstruct: l.ShipInstruct, ShipMode: l.ShipMode, Comment: l.Comment,
			})
		}
		out.Orders[index] = order
	}
	if line != len(flat.Lines) {
		return nil, fmt.Errorf("tpch: %d lines do not follow their order", len(flat.Lines)-line)
	}
	return out, nil
}

// readTable calls row for each line of a dbgen .tbl file, split on '|'. The
// TPC's dbgen ends every line with a '|'; tpch-kit's, built with EOL_HANDLING,
// does not. Either is accepted, and nothing else.
func readTable(path string, columns int, row func([]string) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for number := 1; scanner.Scan(); number++ {
		fields := strings.Split(scanner.Text(), "|")
		if len(fields) == columns+1 && fields[columns] == "" {
			fields = fields[:columns]
		}
		if len(fields) != columns {
			return fmt.Errorf("%s:%d: %d fields, want %d", path, number, len(fields), columns)
		}
		if err := row(fields[:columns]); err != nil {
			return fmt.Errorf("%s:%d: %w", path, number, err)
		}
	}
	return scanner.Err()
}

func parseInt32(s string, err error) (int32, error) {
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err
}

// parseCents reads a DECIMAL(15,2) exactly: dbgen always writes two digits.
func parseCents(s string, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	whole, fraction, ok := strings.Cut(s, ".")
	if !ok || len(fraction) != 2 {
		return 0, fmt.Errorf("not a two-place decimal: %q", s)
	}
	units, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	cents, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, err
	}
	if strings.HasPrefix(whole, "-") {
		return units*100 - cents, nil
	}
	return units*100 + cents, nil
}

func parseDate(s string, err error) (int32, error) {
	if err != nil {
		return 0, err
	}
	t, err := time.Parse(time.DateOnly, s)
	return int32(t.Unix() / 86400), err
}
