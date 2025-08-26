package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rec "simplebankrecon/reconcile"
)

func main() {
	systemPath := flag.String("system", "", "System transaction CSV file path")
	banksArg := flag.String("bank", "", "Comma-separated list of bank statement CSV file paths")
	startStr := flag.String("start", "", "Start date (YYYY-MM-DD) inclusive")
	endStr := flag.String("end", "", "End date (YYYY-MM-DD) inclusive")
	out := flag.String("output-json", "", "Optional path to write JSON summary")
	flag.Parse()

	if *systemPath == "" || *banksArg == "" || *startStr == "" || *endStr == "" {
		flag.Usage()
		os.Exit(2)
	}

	sysRows, err := rec.ParseSystemCSV(*systemPath)
	check(err)

	var banks []rec.BankStatement
	for _, p := range strings.Split(*banksArg, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		inferredBank := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		rows, err := rec.ParseBankCSV(p, inferredBank)
		check(err)
		banks = append(banks, rows...)
	}

	start, err := time.Parse("2006-01-02", *startStr)
	check(err)
	end, err := time.Parse("2006-01-02", *endStr)
	check(err)

	res := rec.Reconcile(sysRows, banks, start, end)

	// Shape an easy-to-consume JSON summary.
	type sysOut struct {
		TrxID           string `json:"trxID"`
		Amount          string `json:"amount"`
		Type            string `json:"type"`
		TransactionTime string `json:"transactionTime"`
	}
	type bankOut struct {
		UniqueIdentifier string `json:"unique_identifier"`
		Amount           string `json:"amount"`
		Date             string `json:"date"`
		Bank             string `json:"bank"`
	}

	unmatchedSys := make([]sysOut, 0, len(res.UnmatchedSystem))
	for _, t := range res.UnmatchedSystem {
		unmatchedSys = append(unmatchedSys, sysOut{
			TrxID:           t.TrxID,
			Amount:          rec.Money(t.AmountCents).String(),
			Type:            string(t.Type),
			TransactionTime: t.TransactionTime.Format(time.RFC3339),
		})
	}

	unmatchedBank := make(map[string][]bankOut, len(res.UnmatchedBankByName))
	unmatchedBankTotal := 0
	for bank, arr := range res.UnmatchedBankByName {
		for _, b := range arr {
			unmatchedBank[bank] = append(unmatchedBank[bank], bankOut{
				UniqueIdentifier: b.UniqueIdentifier,
				Amount:           rec.Money(b.AmountCents).String(),
				Date:             b.Date.Format("2006-01-02"),
				Bank:             b.Bank,
			})
			unmatchedBankTotal++
		}
	}

	summary := map[string]any{
		"total_system_transactions": res.TotalSystemTransactions,
		"total_bank_transactions":   res.TotalBankTransactions,
		"total_processed":           res.TotalProcessed,

		"matched_count":   res.MatchedCount,
		"unmatched_total": len(res.UnmatchedSystem) + unmatchedBankTotal,

		"unmatched_system":       unmatchedSys,
		"unmatched_bank_by_name": unmatchedBank,

		"total_discrepancy": rec.Money(res.TotalDiscrepancyCents).String(),
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(summary)

	if *out != "" {
		f, err := os.Create(*out)
		check(err)
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		_ = enc.Encode(summary)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
