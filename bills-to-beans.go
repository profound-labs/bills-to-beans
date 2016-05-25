package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/codegangsta/negroni"
	"github.com/davecgh/go-spew/spew"
	"github.com/fsnotify/fsnotify"
	"github.com/getwe/figlet4go"
	"github.com/gorilla/mux"
	"github.com/skratchdot/open-golang/open"
	"log"
	"math"
	"os/signal"
	"sort"
	"syscall"
	//"github.com/jung-kurt/gofpdf"
	"github.com/imdario/mergo"
	"gopkg.in/yaml.v2"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	s "strings"
	"time"
)

var developmentMode bool
var useLocal bool

var appTempDir string

type conf struct {
	BillsFolder           string `yaml:"bills_folder"`
	MainBeancountFile     string `yaml:"main_beancount_file"`
	IncludesBeancountFile string `yaml:"includes_beancount_file"`
	ServerPort            int    `yaml:"server_port"`
	InlineBeancounts      bool   `yaml:inline_beancounts`
}

func (c *conf) readConf() *conf {
	theconf := conf{
		BillsFolder:           "./bills",
		MainBeancountFile:     "./bills.beancount",
		IncludesBeancountFile: "./includes.beancount",
		ServerPort:            3030,
		InlineBeancounts:      false,
	}

	yamlFile, err := ioutil.ReadFile("config.yml")
	if err != nil {
		*c = theconf
		return &theconf
	}

	var yamlConf conf

	err = yaml.Unmarshal(yamlFile, &yamlConf)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	mergo.MergeWithOverwrite(&theconf, yamlConf)
	*c = theconf

	return &theconf
}

var config conf

type Document struct {
	Date     time.Time `json:"date"`
	Account  string    `json:"account"`
	Filename string    `json:"filename"`
	DirPath  string
}

type auxiliary_document struct {
	Date     string `json:"date"`
	Account  string `json:"account"`
	Filename string `json:"filename"`
}

type Note struct {
	Date        time.Time `json:"date"`
	Account     string    `json:"account"`
	Description string    `json:"description"`
}

type auxiliary_note struct {
	Date        string `json:"date"`
	Account     string `json:"account"`
	Description string `json:"description"`
}

type Posting struct {
	Flag      string  `json:"flag"`
	Account   string  `json:"account"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	padlength int
}

type auxiliary_posting struct {
	Flag     string `json:"flag"`
	Account  string `json:"account"`
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type Transaction struct {
	Date      time.Time `json:"date"`
	Flag      string    `json:"flag"`
	Payee     string    `json:"payee"`
	Narration string    `json:"narration"`
	Tags      []string  `json:"tags"`
	Link      string    `json:"link"`
	Postings  []Posting `json:"postings"`
}

type auxiliary_transaction struct {
	Date      string              `json:"date"`
	Flag      string              `json:"flag"`
	Payee     string              `json:"payee"`
	Narration string              `json:"narration"`
	Tags      []string            `json:"tags"`
	Link      string              `json:"link"`
	Postings  []auxiliary_posting `json:"postings"`
}

type Balance struct {
	Date          time.Time `json:"date"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	SourceAccount string    `json:"source_account"`
	TargetAccount string    `json:"target_account"`
	Padded        bool      `json:"padded"`
}

type auxiliary_balance struct {
	Date          string `json:"date"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	SourceAccount string `json:"source_account"`
	TargetAccount string `json:"target_account"`
	Padded        bool   `json:"padded"`
}

type Bill struct {
	Transactions []Transaction `json:"transactions"`
	Balances     []Balance     `json:"balances"`
	Documents    []Document    `json:"documents"`
	Notes        []Note        `json:"notes"`
	DirPath      string
}

type auxiliary_bill struct {
	Transactions []auxiliary_transaction `json:"transactions"`
	Balances     []auxiliary_balance     `json:"balances"`
	Documents    []auxiliary_document    `json:"documents"`
	Notes        []auxiliary_note        `json:"notes"`
}

func sanitizeFilename(text string) string {
	portuguese := `ãâáàẽêéèĩîíìõôóòũûúùçÃÂÁÀẼÊÉÈĨÎÍÌÕÔÓÒŨÛÚÙÇ`
	currency := `€£\$`
	out := regexp.MustCompile(`[^\w\.\'`+portuguese+currency+`-]`).ReplaceAllString(text, " ")
	out = regexp.MustCompile(`  +`).ReplaceAllString(out, " ")
	return out
}

// Check if a path exists
// http://stackoverflow.com/a/10510783/195141
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// UniqStr returns a copy of the passed slice with only unique string results.
// http://www.golangbootcamp.com/book/tricks_and_tips
func UniqStr(col []string) []string {
	m := map[string]struct{}{}
	for _, v := range col {
		if _, ok := m[v]; !ok {
			m[v] = struct{}{}
		}
	}
	list := make([]string, len(m))

	i := 0
	for v := range m {
		list[i] = v
		i++
	}
	return list
}

func figletString(text string) string {
	ascii := figlet4go.NewAsciiRender()
	renderStr, _ := ascii.Render(text)
	return renderStr
}

func (n Note) String() string {
	return fmt.Sprintf(
		"%s note %s %q",
		n.Date.Format("2006-01-02"),
		n.Account,
		n.Description,
	)
}

func (d Document) String() string {
	return fmt.Sprintf(
		"%s document %s %q",
		d.Date.Format("2006-01-02"),
		d.Account,
		// expected to be relative, written for the document in the same folder as
		// the beancount file
		filepath.Join(d.Filename),
	)
}

// http://stackoverflow.com/a/21061062/195141
func (d Document) Copy(dst string) error {
	in, err := os.Open(filepath.Join(appTempDir, d.Filename))
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}

func (b Balance) String() string {
	var out []string

	if b.Padded {
		out = append(out,
			fmt.Sprintf(
				"%s pad %s %s",
				b.Date.Format("2006-01-02"),
				b.SourceAccount,
				b.TargetAccount,
			))
	}

	out = append(out,
		fmt.Sprintf(
			"%s balance %s %.2f %s",
			b.Date.Format("2006-01-02"),
			b.SourceAccount,
			b.Amount,
			b.Currency,
		))

	return s.Join(out, "\n\n")
}

func (p Posting) accFmt() string {
	out := fmt.Sprintf("%s %s", p.Flag, p.Account)
	out = regexp.MustCompile(`  +`).ReplaceAllString(out, " ")
	out = s.TrimSpace(out)
	return out
}

func (p Posting) String() string {
	var out string

	out = p.accFmt()

	if p.padlength > 0 {
		for i := 0; len(out) < p.padlength; i++ {
			out = out + " "
		}
	}

	if p.Amount != 0.0 && len(p.Currency) > 0 {
		out = out + fmt.Sprintf(" %.2f %s", p.Amount, p.Currency)
	}
	return out
}

func (t Transaction) titleFmt() string {
	payee := s.Replace(t.Payee, `"`, `'`, -1)
	narration := s.Replace(t.Narration, `"`, `'`, -1)

	if len(payee) == 0 && len(narration) == 0 {
		return ""
	}

	// Fava adds a | between payee and narration:
	// [[Payee] \| Narration]

	// if there is a payee, there must be a narration too
	if len(payee) > 0 {
		return fmt.Sprintf(`"%s" | "%s"`, payee, narration)
	}

	return fmt.Sprintf(`"%s"`, narration)
}

func (t Transaction) flagFmt() string {
	if len(t.Flag) == 0 {
		return "*"
	}
	if t.Flag == "*" || t.Flag == "!" {
		return t.Flag
	}
	return "*"
}

func (t Transaction) String() string {
	out := ""

	firstLineParts := []string{
		t.Date.Format("2006-01-02"),
		t.flagFmt(),
		t.titleFmt(),
		s.Join(t.Tags, " "),
		t.Link,
	}

	out = out + regexp.MustCompile(`  +`).ReplaceAllString(s.Join(firstLineParts, " "), " ")
	out = s.TrimSpace(out)

	longest := 0
	for _, p := range t.Postings {
		if len(p.accFmt()) > longest {
			longest = len(p.accFmt())
		}
	}

	for _, p := range t.Postings {
		p.padlength = longest + 1
		if p.Amount >= 0 {
			p.padlength++
		}
		out = out + fmt.Sprintf("\n  %s", p.String())
	}

	return out
}

func (t Transaction) sumAmountFmt() string {
	// No postings, 0 amount
	// TODO have config for default currency
	if len(t.Postings) == 0 {
		return "€0.00"
	}

	// 1 or 2 postings, take abs of first for the amount
	if len(t.Postings) <= 2 {
		currency := ""
		switch t.Postings[0].Currency {
		case "EUR":
			currency = "€"
		case "GBP":
			currency = "£"
		case "USD":
			currency = "$"
		}

		return fmt.Sprintf("%s%.2f", currency, math.Abs(t.Postings[0].Amount))
	}
	return ""
}

func (note Note) sanitizedBase() string {
	parts := []string{
		note.Date.Format("2006-01-02"),
		"note",
	}
	return sanitizeFilename(s.Join(parts, " _ "))
}

func (bal Balance) sanitizedBase() string {
	parts := []string{
		bal.Date.Format("2006-01-02"),
		"balance",
	}
	return sanitizeFilename(s.Join(parts, " _ "))
}

func (t Transaction) sanitizedBase() string {
	var parts []string
	if len(t.Payee) > 0 {
		parts = []string{
			t.Date.Format("2006-01-02"),
			t.Payee,
			t.Narration,
			t.sumAmountFmt(),
		}
	} else {
		parts = []string{
			t.Date.Format("2006-01-02"),
			t.Narration,
			t.sumAmountFmt(),
		}
	}
	return sanitizeFilename(s.Join(parts, " _ "))
}

func (b Bill) String() string {
	var strs []string

	for _, txn := range b.Transactions {
		strs = append(strs, txn.String())
	}

	for _, bal := range b.Balances {
		strs = append(strs, bal.String())
	}

	for _, note := range b.Notes {
		strs = append(strs, note.String())
	}

	// TODO front-end will have to fill in missing account and date info
	//for _, doc := range b.Documents {
	//	doc.DirPath = b.DirPath
	//	strs = append(strs, doc.String())
	//}

	return s.Join(strs, "\n\n")
}

// Uses globals: config
func (b *Bill) EnsureDirPath() error {
	var err error

	// Use the first Transaction or Balance

	if len(b.Transactions) > 0 {
		t := b.Transactions[0]
		b.DirPath = filepath.Join(
			config.BillsFolder,
			fmt.Sprintf("%04d", t.Date.Year()),
			fmt.Sprintf("%02d", t.Date.Month()),
			t.sanitizedBase(),
		)
	} else if len(b.Balances) > 0 {
		bal := b.Balances[0]
		b.DirPath = filepath.Join(
			config.BillsFolder,
			fmt.Sprintf("%04d", bal.Date.Year()),
			fmt.Sprintf("%02d", bal.Date.Month()),
			bal.sanitizedBase(),
		)
	} else if len(b.Notes) > 0 {
		note := b.Notes[0]
		b.DirPath = filepath.Join(
			config.BillsFolder,
			fmt.Sprintf("%04d", note.Date.Year()),
			fmt.Sprintf("%02d", note.Date.Month()),
			note.sanitizedBase(),
		)
	} else {
		return errors.New(fmt.Sprintf("Need at least one transaction, balance or note"))
	}

	if ex, _ := exists(b.DirPath); ex {
		return errors.New(fmt.Sprintf("Already exists: %s", b.DirPath))
	}

	if err = os.MkdirAll(b.DirPath, 0755); err != nil {
		return err
	}

	return nil
}

func (b Bill) BeancountFilename() string {
	return "bill.beancount"
}

func (b *Bill) SaveBeancount() error {
	var err error

	path := filepath.Join(b.DirPath, b.BeancountFilename())

	if ex, _ := exists(path); ex {
		return errors.New(fmt.Sprintf("Already exists: %s", path))
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(b.String())
	if err != nil {
		return err
	}

	return nil
}

func (t *Transaction) ParseBeancount(text string) error {
	text = s.TrimSpace(text)
	firstLine := s.TrimSpace(s.Split(text, "\n")[0])

	re := regexp.MustCompile(`^([^ ]+) ([\*\!]) *("[^"]+")?[ \|]*("[^"]+")?`)
	matches := re.FindStringSubmatch(firstLine)

	if len(matches) > 0 {
		t.Date, _ = time.Parse("2006-01-02", matches[1])
		t.Flag = matches[2]
		// matches[0] is the complete matched string
		if len(matches[4]) > 0 {
			t.Payee = s.Trim(matches[3], `"`)
			t.Narration = s.Trim(matches[4], `"`)
		} else {
			t.Narration = s.Trim(matches[3], `"`)
		}
	} else {
		return errors.New("no matches")
	}

	re = regexp.MustCompile(`#[\w-]+`)
	matches = re.FindAllString(firstLine, -1)
	if matches != nil {
		t.Tags = matches
	}

	re = regexp.MustCompile(`\^[\w-]+`)
	match := re.FindString(firstLine)
	if len(match) != 0 {
		t.Link = match
	}

	return nil
}

func (b *Bill) SaveDocuments() (err error) {
	for _, doc := range b.Documents {
		if len(doc.Filename) == 0 {
			continue
		}
		newpath := filepath.Join(b.DirPath, doc.Filename)
		ex, err := exists(newpath)
		if ex {
			return errors.New(fmt.Sprintf("File already exists: %s", newpath))
		}
		if err != nil {
			return err
		}
		err = doc.Copy(newpath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Bill) Save(c conf) error {
	var err error

	// be create about new folders, don't just error out
	// rather duplicate than delete

	if err = b.EnsureDirPath(); err != nil {
		return err
	}

	if err = b.SaveBeancount(); err != nil {
		return err
	}

	if err = b.SaveDocuments(); err != nil {
		return err
	}

	if err = c.updateIncludesBeancountFile(); err != nil {
		return err
	}

	return nil
}

func (aux_bal auxiliary_balance) ToBalance() Balance {
	var date time.Time
	if len(aux_bal.Date) >= 10 {
		date, _ = time.Parse("2006-01-02", aux_bal.Date[0:10])
	} else {
		date = time.Now()
	}

	amount, _ := strconv.ParseFloat(aux_bal.Amount, 64)

	bal := Balance{
		Date:          date,
		Amount:        amount,
		Currency:      aux_bal.Currency,
		SourceAccount: aux_bal.SourceAccount,
		//TargetAccount:  aux_bal.TargetAccount,
		//Padded: aux_bal.Padded,
	}

	return bal
}

func isostrToDate(text string) time.Time {
	var date time.Time
	if len(text) >= 10 {
		date, _ = time.Parse("2006-01-02", text[0:10])
	} else {
		date = time.Now()
	}
	return date
}

func (aux_note auxiliary_note) ToNote() Note {
	return Note{
		Date:        isostrToDate(aux_note.Date),
		Account:     aux_note.Account,
		Description: aux_note.Description,
	}
}

func (aux_doc auxiliary_document) ToDocument() Document {
	return Document{
		Date:     isostrToDate(aux_doc.Date),
		Account:  aux_doc.Account,
		Filename: aux_doc.Filename,
	}
}

func (aux_txn auxiliary_transaction) ToTransaction() Transaction {
	txn := Transaction{
		Date:      isostrToDate(aux_txn.Date),
		Flag:      aux_txn.Flag,
		Payee:     s.Replace(aux_txn.Payee, `"`, `'`, -1),
		Narration: s.Replace(aux_txn.Narration, `"`, `'`, -1),
	}

	for _, p := range aux_txn.Postings {
		amount, _ := strconv.ParseFloat(p.Amount, 64)
		txn.Postings = append(txn.Postings,
			Posting{
				Flag:     p.Flag,
				Account:  p.Account,
				Amount:   amount,
				Currency: p.Currency,
			},
		)
	}

	return txn
}

func MyClassic() *negroni.Negroni {
	return negroni.New(negroni.NewRecovery(), negroni.NewLogger(), negroni.NewStatic(Dir(useLocal, "/public")))
}

// GetLocalIP returns the non loopback local IP of the host
// http://stackoverflow.com/a/31551220/195141
// could also do https://www.socketloop.com/tutorials/golang-how-do-I-get-the-local-ip-non-loopback-address
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil && int(ipnet.IP.To4()[0]) == 192 {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func sendError(w http.ResponseWriter, err error) {
	data := make(map[string]interface{})
	msg := fmt.Sprintf("%v", err)
	log.Println(msg)
	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	data["flash"] = msg
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]string)
	data["localAddress"] = fmt.Sprintf("http://%s:%d", GetLocalIP(), config.ServerPort)

	t, _ := template.New("index").Parse(FSMustString(useLocal, "/public/index.html.tmpl"))
	t.Execute(w, data)
}

// Uses globals: appTempDir
func createNewTempdir(w http.ResponseWriter, r *http.Request) {
	os.RemoveAll(appTempDir)
	appTempDir, _ = ioutil.TempDir(os.TempDir(), "bills_")
}

// Uses globals: appTempDir
func removeFromTempdir(w http.ResponseWriter, r *http.Request) {
	var err error

	if err = r.ParseForm(); err != nil {
		sendError(w, errors.New("Could not remove file"))
		return
	}

	filename := r.PostFormValue("filename")

	if err = os.Remove(filepath.Join(appTempDir, filename)); err != nil {
		sendError(w, errors.New(fmt.Sprintf("Could not remove file: %s", filename)))
		return
	}
}

// Uses globals: config, appTempDir
func saveBillHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	// Have to decode with auxiliary structs b/c date and numbers come as strings
	// idea is from https://mlafeldt.github.io/blog/decoding-yaml-in-go/

	var aux_bill auxiliary_bill

	if err := decoder.Decode(&aux_bill); err != nil {
		sendError(w, err)
		return
	}

	var bill Bill

	// Documents

	for _, aux_doc := range aux_bill.Documents {
		doc := aux_doc.ToDocument()
		bill.Documents = append(bill.Documents, doc)
	}

	// Transactions

	for _, aux_txn := range aux_bill.Transactions {
		txn := aux_txn.ToTransaction()
		bill.Transactions = append(bill.Transactions, txn)
	}

	// Balances

	for _, aux_bal := range aux_bill.Balances {
		bal := aux_bal.ToBalance()
		bill.Balances = append(bill.Balances, bal)
	}

	// Notes

	for _, aux_note := range aux_bill.Notes {
		note := aux_note.ToNote()
		bill.Notes = append(bill.Notes, note)
	}

	if err := bill.Save(config); err != nil {
		sendError(w, err)
		return
	}

	os.RemoveAll(appTempDir)
	appTempDir, _ = ioutil.TempDir(os.TempDir(), "bills_")

	data := make(map[string]interface{})
	data["flash"] = "Saved"

	data["dir_path"] = bill.DirPath

	// can't do structs with keys, it only encodes as empty hashes
	savedpaths := []string{}
	savedsizes := []int64{}

	filepath.Walk(
		bill.DirPath,
		func(path string, f os.FileInfo, err error) error {
			if !f.IsDir() {
				savedpaths = append(savedpaths, path)
				savedsizes = append(savedsizes, f.Size())
			}
			return nil
		},
	)

	data["saved_paths"] = savedpaths
	data["saved_sizes"] = savedsizes

	enc := json.NewEncoder(w)
	w.Header().Set("Content-type", "application/json")
	enc.Encode(data)
}

func (c conf) getAccounts() (account []string, err error) {
	content, err := ioutil.ReadFile(c.MainBeancountFile)
	if err != nil {
		return []string{}, err
	}

	data := []string{}
	var matches [][]string

	re := regexp.MustCompile(`\n[^ ]+ +open +([^ \n]+)`)

	matches = re.FindAllStringSubmatch(string(content), -1)

	for _, m := range matches {
		data = append(data, m[1])
	}

	return data, nil
}

func (c conf) getCurrencies() (currencies []string, err error) {
	content, err := ioutil.ReadFile(c.MainBeancountFile)
	if err != nil {
		return []string{}, err
	}

	data := []string{}
	var matches [][]string

	re := regexp.MustCompile(`\noption "operating_currency" +"([^ \n]+)"`)

	matches = re.FindAllStringSubmatch(string(content), -1)

	for _, m := range matches {
		data = append(data, m[1])
	}

	return data, nil
}

// Uses globals: config
func completionsHandler(w http.ResponseWriter, r *http.Request) {
	globpath := filepath.Join(config.BillsFolder, "*", "*", "*", "*.beancount")
	paths, _ := filepath.Glob(globpath)

	data := make(map[string][]string)

	data["payees"] = []string{}
	data["tags"] = []string{}
	data["links"] = []string{}
	data["accounts"] = []string{}
	data["currencies"] = []string{}

	for _, path := range paths {
		c, _ := ioutil.ReadFile(path)
		text := string(c)
		txn := Transaction{}
		if err := txn.ParseBeancount(text); err != nil {
			log.Printf("%v", err)
		} else {
			if len(txn.Payee) > 0 {
				data["payees"] = append(data["payees"], txn.Payee)
			}
			if len(txn.Link) > 0 {
				data["links"] = append(data["links"], txn.Link)
			}
			if len(txn.Tags) > 0 {
				for _, t := range txn.Tags {
					data["tags"] = append(data["tags"], t)
				}
			}
		}
	}

	data["payees"] = UniqStr(data["payees"])
	data["tags"] = UniqStr(data["tags"])
	data["links"] = UniqStr(data["links"])

	sort.Sort(sort.StringSlice(data["payees"]))
	sort.Sort(sort.StringSlice(data["tags"]))
	sort.Sort(sort.StringSlice(data["links"]))

	accounts, err := config.getAccounts()
	if err != nil {
		sendError(w, err)
		return
	}

	data["accounts"] = accounts

	currencies, err := config.getCurrencies()
	if err != nil {
		sendError(w, err)
		return
	}

	data["currencies"] = currencies

	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	r.ParseMultipartForm(32 << 20) // using 32 MB memory

	file, handler, err := r.FormFile("file")
	if err != nil {
		sendError(w, err)
		return
	}
	defer file.Close()

	//ct := handler.Header.Get("Content-Type")
	//
	//if !(ct == "image/png" || ct == "image/jpeg") {
	//	err = errors.New("must be png or jpeg")
	//	fmt.Println(err)
	//	return
	//}

	path := filepath.Join(appTempDir, handler.Filename)

	if ex, _ := exists(path); ex {
		sendError(w, errors.New(fmt.Sprintf("Already exists: %s", path)))
		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		sendError(w, err)
		return
	}
	defer f.Close()

	if _, err = io.Copy(f, file); err != nil {
		sendError(w, err)
		return
	}

	data := make(map[string]interface{})
	info, _ := f.Stat()
	data["filename"] = filepath.Base(path)
	data["size"] = info.Size()

	// Simulate waiting time for upload during development
	if developmentMode {
		time.Sleep(time.Duration(2) * time.Second)
	}

	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

// uses globals: config
func (c conf) updateIncludesBeancountFile() error {
	var err error

	globpath := filepath.Join(config.BillsFolder, "*", "*", "*", "*.beancount")
	paths, _ := filepath.Glob(globpath)

	var billTexts []string
	var content []byte
	var text string

	for _, path := range paths {
		if c.InlineBeancounts {
			content, _ = ioutil.ReadFile(path)
			text = string(content) + "\n"
		} else {
			relpath, _ := filepath.Rel(filepath.Dir(config.IncludesBeancountFile), path)
			text = fmt.Sprintf(`include %q`, relpath)
		}
		billTexts = append(billTexts, text)
	}

	// don't check if exists, overwrite
	f, err := os.OpenFile(config.IncludesBeancountFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	out := s.Join(billTexts, "\n")
	f.Write([]byte(out))

	return nil
}

func (c conf) startWebApp() {
	port := c.ServerPort
	router := mux.NewRouter()

	router.HandleFunc("/", indexHandler).Methods("GET")

	router.HandleFunc("/save-bill", saveBillHandler).Methods("POST")
	router.HandleFunc("/upload", uploadHandler).Methods("POST")

	router.HandleFunc("/new-tempdir", createNewTempdir).Methods("POST")
	router.HandleFunc("/remove-from-tempdir", removeFromTempdir).Methods("POST")

	router.HandleFunc("/completions.json", completionsHandler).Methods("GET")

	n := MyClassic()
	n.UseHandler(router)

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
		// just to keep spew in the imports
		spew.Dump(err)
	}

	// Print welcome message
	fmt.Println(figletString("B2B"))
	fmt.Printf("Listening on http://localhost:%d\n", port)

	config.openBrowser()

	// Start the blocking server loop.
	log.Fatal(http.Serve(l, n))
}

func (c conf) openBrowser() {
	if developmentMode {
		return
	}
	err := open.Start(fmt.Sprintf("http://localhost:%d", c.ServerPort))
	if err != nil {
		log.Println(err)
	}
}

func cleanup() {
	log.Printf("removing app temp folder %s", appTempDir)
	os.RemoveAll(appTempDir)
}

// uses globals: config
func actionWatch(c *cli.Context) error {
	var err error
	fmt.Println(figletString("WATCH"))

	log.Printf("Updating %s\n", config.IncludesBeancountFile)
	if err = config.updateIncludesBeancountFile(); err != nil {
		log.Println(err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if path.Ext(event.Name) == ".beancount" && (event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove) {

					var word string

					if event.Op&fsnotify.Create == fsnotify.Create {
						word = "create"
					}
					if event.Op&fsnotify.Remove == fsnotify.Remove {
						word = "remove"
					}

					log.Printf("File %s event: %s\n", word, event.Name)
					log.Printf("Updating %s\n", config.IncludesBeancountFile)
					if err = config.updateIncludesBeancountFile(); err != nil {
						log.Println(err)
					}

				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	fmt.Printf("Watching %s for changes...\n", config.BillsFolder)

	filepath.Walk(
		config.BillsFolder,
		func(path string, f os.FileInfo, err error) error {
			if f.IsDir() {
				if err = watcher.Add(path); err != nil {
					log.Fatal(err)
				}
			}
			return nil
		},
	)

	<-done

	return nil
}

func main() {
	var err error

	app := cli.NewApp()
	app.Name = "bills-to-beans"
	app.Usage = "helper app to record bills in beancount format"

	config.readConf()

	if os.Getenv("ENV") == "development" {
		developmentMode = true
	} else {
		developmentMode = false
	}
	if developmentMode {
		useLocal = true
	} else {
		useLocal = false
	}

	appTempDir, err = ioutil.TempDir(os.TempDir(), "bills_")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Cleanup on exit or Ctrl-C interrupt
	// http://stackoverflow.com/a/18158859/195141
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		<-c
		cleanup()
		os.Exit(1)
	}()

	app.Commands = []cli.Command{
		{
			Name:   "watch",
			Usage:  "watch the bills folder for changes and update the includes file",
			Action: actionWatch,
		},
	}

	app.Action = func(c *cli.Context) error {
		// No arguments, so we're a desktop web app
		if c.NArg() < 1 {
			if err = config.updateIncludesBeancountFile(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			config.startWebApp()
		}

		return nil
	}

	app.Run(os.Args)
}
