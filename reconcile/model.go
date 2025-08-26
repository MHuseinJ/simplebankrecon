package reconcile

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// TxType is the transaction type.
type TxType string

const (
	DEBIT  TxType = "DEBIT"
	CREDIT TxType = "CREDIT"
)

// Money uses integer cents for exact currency math (no float drift).
type Money int64

func (m Money) String() string {
	sign := ""
	v := m
	if m < 0 {
		sign = "-"
		v = -m
	}
	dollars := int64(v) / 100
	cents := int64(v) % 100
	return fmt.Sprintf("%s%d.%02d", sign, dollars, cents)
}

// Transaction (system) - time is RFC3339; matching happens at DATE granularity.
type Transaction struct {
	TrxID           string
	AmountCents     Money
	Type            TxType
	TransactionTime time.Time
}

func (t Transaction) SignedAmount() Money {
	if t.Type == CREDIT {
		return t.AmountCents
	}
	return -t.AmountCents
}

func (t Transaction) DateOnly() time.Time {
	// Truncate to midnight for date-based matching.
	y, m, d := t.TransactionTime.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.TransactionTime.Location())
}

// BankStatement row (bank may vary per file).
type BankStatement struct {
	UniqueIdentifier string
	AmountCents      Money // can be negative for debits
	Date             time.Time
	Bank             string
}

// -------- CSV loaders (simple & explicit) --------

func ParseSystemCSV(path string) ([]Transaction, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	head, err := r.Read()
	if err != nil {
		return nil, err
	}

	idx := func(name string) int {
		for i, h := range head {
			if strings.EqualFold(strings.TrimSpace(h), name) {
				return i
			}
		}
		return -1
	}

	iTrx := idx("trxID")
	iAmt := idx("amount")
	iType := idx("type")
	iTime := idx("transactionTime")
	if iTrx < 0 || iAmt < 0 || iType < 0 || iTime < 0 {
		return nil, errors.New("missing required headers in system CSV (trxID,amount,type,transactionTime)")
	}

	var out []Transaction
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		amt, err := parseDecimalToCents(rec[iAmt])
		if err != nil {
			return nil, fmt.Errorf("amount parse: %w", err)
		}

		typeStr := strings.ToUpper(strings.TrimSpace(rec[iType]))
		if typeStr != string(DEBIT) && typeStr != string(CREDIT) {
			return nil, fmt.Errorf("invalid type: %s", typeStr)
		}

		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(rec[iTime]))
		if err != nil {
			return nil, fmt.Errorf("transactionTime parse (RFC3339): %w", err)
		}

		out = append(out, Transaction{
			TrxID:           strings.TrimSpace(rec[iTrx]),
			AmountCents:     Money(amt),
			Type:            TxType(typeStr),
			TransactionTime: ts,
		})
	}

	return out, nil
}

func ParseBankCSV(path string, bankName string) ([]BankStatement, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	head, err := r.Read()
	if err != nil {
		return nil, err
	}

	idx := func(name string) int {
		for i, h := range head {
			if strings.EqualFold(strings.TrimSpace(h), name) {
				return i
			}
		}
		return -1
	}

	iUID := idx("unique_identifier")
	iAmt := idx("amount")
	iDate := idx("date")
	iBank := idx("bank")
	if iUID < 0 || iAmt < 0 || iDate < 0 {
		return nil, errors.New("missing required headers in bank CSV (unique_identifier,amount,date[,bank])")
	}

	var out []BankStatement
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		amt, err := parseDecimalToCents(rec[iAmt])
		if err != nil {
			return nil, fmt.Errorf("amount parse: %w", err)
		}
		d, err := time.Parse("2006-01-02", strings.TrimSpace(rec[iDate]))
		if err != nil {
			return nil, fmt.Errorf("date parse (YYYY-MM-DD): %w", err)
		}

		b := strings.TrimSpace(bankName)
		if iBank >= 0 && strings.TrimSpace(rec[iBank]) != "" {
			b = strings.TrimSpace(rec[iBank])
		}
		if b == "" {
			b = "UNKNOWN"
		}

		out = append(out, BankStatement{
			UniqueIdentifier: strings.TrimSpace(rec[iUID]),
			AmountCents:      Money(amt),
			Date:             d,
			Bank:             b,
		})
	}

	return out, nil
}

// parseDecimalToCents parses "-100.25" into -10025 (cents). Truncates extra precision.
func parseDecimalToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty amount")
	}
	sign := int64(1)
	if s[0] == '-' {
		sign = -1
		s = s[1:]
	} else if s[0] == '+' {
		s = s[1:]
	}

	parts := strings.SplitN(s, ".", 3)
	if len(parts) == 1 {
		v, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, err
		}
		return sign * v * 100, nil
	}

	dollars := parts[0]
	cents := parts[1]
	if len(cents) > 2 {
		cents = cents[:2] // truncate while loading
	}
	for len(cents) < 2 {
		cents += "0"
	}
	vd, err := strconv.ParseInt(dollars, 10, 64)
	if err != nil {
		return 0, err
	}
	vc, err := strconv.ParseInt(cents, 10, 64)
	if err != nil {
		return 0, err
	}

	return sign * (vd*100 + vc), nil
}
