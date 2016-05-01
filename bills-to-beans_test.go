package main

import (
	//"github.com/davecgh/go-spew/spew"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func isodate(str string) time.Time {
	d, _ := time.Parse("2006-01-02", str)
	return d
}

func TestBalanceString(t *testing.T) {
	var balances = map[Balance]string{

		Balance{
			Date:          isodate("2016-03-21"),
			Amount:        324.25,
			Currency:      "EUR",
			SourceAccount: "Assets:Bank:Checking",
			Padded:        false,
		}: "2016-03-21 balance Assets:Bank:Checking 324.25 EUR",

		Balance{
			Date:          isodate("2016-03-21"),
			Amount:        324.25,
			Currency:      "EUR",
			SourceAccount: "Assets:Bank:Checking",
			TargetAccount: "Equity:Opening-Balances",
			Padded:        true,
		}: `2016-03-21 pad Assets:Bank:Checking Equity:Opening-Balances

2016-03-21 balance Assets:Bank:Checking 324.25 EUR`,
	}

	for balance, expect := range balances {
		res := balance.String()
		if res != expect {
			t.Errorf("hey: %s", res)
		}
	}
}

func TestTransactionString(t *testing.T) {
	var expect string
	var res string
	var transaction Transaction

	transaction = Transaction{
		Date:      isodate("2016-02-12"),
		Payee:     `Café de 'João'`,
		Narration: `dois "X" café por cabeça`,
		Tags:      []string{"#coffee", "#portugal"},
		Link:      "^holiday-2016",
		Postings: []Posting{
			Posting{Account: "Assets:Bank:Checking", Amount: -5.50, Currency: "EUR"},
			Posting{Account: "Expenses:Coffee"},
		},
	}

	expect = `2016-02-12 * "Café de 'João'" "dois 'X' café por cabeça" #coffee #portugal ^holiday-2016
Assets:Bank:Checking -5.50 EUR
Expenses:Coffee`

	res = transaction.String()

	if res != expect {
		t.Errorf("hey: %s", res)
	}
}

func TestDocumentString(t *testing.T) {
	documents := map[Document]string{
		Document{
			Date:    isodate("2015-08-11"),
			Account: "Assets:Bank:Checking",
			Path:    "./scans.pdf",
		}: `2015-08-11 document Assets:Bank:Checking "./scans.pdf"`,
	}

	for doc, expect := range documents {
		res := doc.String()
		if res != expect {
			t.Errorf("hey: %s", res)
		}
	}
}

func TestSanitizedBase(t *testing.T) {
	txn := Transaction{
		Date:      isodate("2016-02-12"),
		Payee:     `Café de 'João'`,
		Narration: `dois "X" café por cabeça`,
		Postings: []Posting{
			Posting{Account: "Assets:Bank:Checking", Amount: -5.50, Currency: "EUR"},
			Posting{Account: "Expenses:Coffee"},
		},
	}

	expect := `2016-02-12 _ Café de 'João' _ dois X café por cabeça _ -5.50 EUR`
	res := txn.sanitizedBase()

	if res != expect {
		t.Errorf("hey: %s", res)
	}

	txn = Transaction{
		Date:      isodate("2016-02-12"),
		Narration: `dois "X" café por cabeça`,
		Postings: []Posting{
			Posting{Account: "Assets:Bank:Checking", Amount: -5.50, Currency: "EUR"},
			Posting{Account: "Expenses:Coffee"},
		},
	}

	expect = `2016-02-12 _ dois X café por cabeça _ -5.50 EUR`
	res = txn.sanitizedBase()

	if res != expect {
		t.Errorf("hey: %s", res)
	}
}

func TestTransactionSave(t *testing.T) {
	transaction := Transaction{
		Date:      isodate("2016-02-12"),
		Payee:     `Café de 'João'`,
		Narration: `dois "X" café por cabeça`,
		Tags:      []string{"coffee", "portugal"},
		Link:      "holiday-2016",
		Postings: []Posting{
			Posting{Account: "Assets:Bank:Checking", Amount: -5.50, Currency: "EUR"},
			Posting{Account: "Expenses:Coffee"},
		},
		Documents: []Document{
			Document{Path: "./testdata/bill-one.png", Saved: false},
			Document{Path: "./testdata/bill-two.jpg", Saved: false},
			Document{Path: "./testdata/some-doc.pdf", Saved: false},
		},
	}

	config.BillsFolder = "./testbills"
	defer os.RemoveAll(config.BillsFolder)

	transaction.Save()

	// there should be a beancount file
	path := filepath.Join(transaction.DirPath(), transaction.BeancountFilename())

	text, err := ioutil.ReadFile(path)
	if err != nil {
		t.Errorf("hey: %v", err)
	}

	if string(text) != transaction.String() {
		t.Errorf("hey: %s", text)
	}

	// there should be three documents

	count := 0

	for _, doc := range transaction.Documents {
		f, err := os.Open(filepath.Join(transaction.DirPath(), filepath.Base(doc.Path)))
		defer f.Close()
		if err != nil {
			t.Errorf("hey: %v", err)
			break
		}
		count++
	}

	if count != len(transaction.Documents) {
		t.Errorf("hey: only %d out of %d documents were saved", count, len(transaction.Documents))
	}
}

func TestParseBeancount(t *testing.T) {
	var txn Transaction
	var text string

	text = `2016-02-12 * "Café de 'João'" "dois 'X' café por cabeça" #coffee #portugal ^holiday-2016
Assets:Bank:Checking -5.50 EUR
Expenses:Coffee`

	txn.ParseBeancount(text)

	switch {
	case txn.Payee != "Café de 'João'",
		txn.Narration != "dois 'X' café por cabeça",
		txn.Tags[0] != "#coffee",
		txn.Tags[1] != "#portugal",
		txn.Link != "^holiday-2016":
		t.Errorf("hey: %v", txn)
	}

	text = `2016-02-12 * "dois 'X' café por cabeça" #coffee #portugal ^holiday-2016
Assets:Bank:Checking -5.50 EUR
Expenses:Coffee`

	txn = Transaction{}
	txn.ParseBeancount(text)

	if txn.Narration != "dois 'X' café por cabeça" {
		t.Errorf("hey: %v", txn)
	}

}

//func TestCompletions(t *testing.T) {
//	config.readConf()
//	globpath := filepath.Join(config.BillsFolder, "*", "*", "*", "*.beancount")
//	//spew.Dump(globpath)
//	files, _ := filepath.Glob(globpath)
//	//spew.Dump(files)
//}
