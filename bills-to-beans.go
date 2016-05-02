package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/codegangsta/negroni"
	//"github.com/davecgh/go-spew/spew"
	"github.com/getwe/figlet4go"
	"github.com/gorilla/mux"
	"github.com/skratchdot/open-golang/open"
	"log"
	"math"
	"os/signal"
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
	BillsFolder       string `yaml:"bills_folder"`
	MainBeancountFile string `yaml:"main_beancount_file"`
	ServerPort        int    `yaml:"server_port"`
}

func (c *conf) readConf() *conf {
	theconf := conf{
		BillsFolder:       "./bills",
		MainBeancountFile: "./bills.beancount",
		ServerPort:        3030,
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
	Date    time.Time `json:"date"`
	Account string    `json:"account"`
	Path    string    `json:"path"`
}

type Posting struct {
	Flag     string  `json:"flag"`
	Account  string  `json:"account"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type TxnDocument struct {
	Filename string `json:"filename"`
}

type Transaction struct {
	Date      time.Time     `json:"date"`
	Flag      string        `json:"flag"`
	Payee     string        `json:"payee"`
	Narration string        `json:"narration"`
	Tags      []string      `json:"tags"`
	Link      string        `json:"link"`
	Postings  []Posting     `json:"postings"`
	Documents []TxnDocument `json:"documents"`
	DirPath   string
}

type Balance struct {
	Date          time.Time `json:"date"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	SourceAccount string    `json:"source_account"`
	TargetAccount string    `json:"target_account"`
	Padded        bool      `json:"padded"`
}

func sanitizeFilename(text string) string {
	out := regexp.MustCompile(`[^\w\.\'ãÃáÁíÍêÊéÉçÇ€£\$-]`).ReplaceAllString(text, " ")
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

func (d Document) String() string {
	return fmt.Sprintf(
		"%s document %s %s",
		d.Date.Format("2006-01-02"),
		d.Account,
		`"`+d.Path+`"`,
	)
}

// http://stackoverflow.com/a/21061062/195141
func (d TxnDocument) Copy(dst string) error {
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

func (p Posting) String() string {
	out := fmt.Sprintf("%s %s", p.Flag, p.Account)
	if p.Amount != 0.0 && len(p.Currency) > 0 {
		out = out + fmt.Sprintf(" %.2f %s", p.Amount, p.Currency)
	}
	out = regexp.MustCompile(`  +`).ReplaceAllString(out, " ")
	out = s.TrimSpace(out)
	return out
}

func (t Transaction) titleFmt() string {
	var out []string
	if len(t.Payee) > 0 {
		out = append(out, `"`+s.Replace(t.Payee, `"`, `'`, -1)+`"`)
	}
	if len(t.Narration) > 0 {
		out = append(out, `"`+s.Replace(t.Narration, `"`, `'`, -1)+`"`)
	}
	return s.Join(out, " ")
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

	for _, p := range t.Postings {
		out = out + "\n" + p.String()
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

func (t *Transaction) EnsureDirPath() error {
	var err error

	t.DirPath = filepath.Join(
		config.BillsFolder,
		fmt.Sprintf("%04d", t.Date.Year()),
		fmt.Sprintf("%02d", t.Date.Month()),
		t.sanitizedBase(),
	)

	if ex, _ := exists(t.DirPath); ex {
		return errors.New(fmt.Sprintf("Already exists: %s", t.DirPath))
	}

	if err = os.MkdirAll(t.DirPath, 0755); err != nil {
		return err
	}

	return nil
}

func (t Transaction) BeancountFilename() string {
	return "transaction.beancount"
}

func (t *Transaction) SaveBeancount() error {
	var err error

	path := filepath.Join(t.DirPath, t.BeancountFilename())

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(t.String())
	if err != nil {
		return err
	}

	return nil
}

func (t *Transaction) ParseBeancount(text string) error {
	text = s.TrimSpace(text)
	firstLine := s.TrimSpace(s.Split(text, "\n")[0])

	re := regexp.MustCompile(`^([^ ]+) ([\*\!]) +("[^"]+")? +("[^"]+")?`)
	matches := re.FindStringSubmatch(firstLine)

	if len(matches) > 0 {
		t.Date, _ = time.Parse("2006-01-02", matches[1])
		t.Flag = matches[2]
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

func (t *Transaction) SaveTxnDocuments() error {
	var err error

	for _, doc := range t.Documents {
		// TODO check if filename already exists
		newpath := filepath.Join(t.DirPath, doc.Filename)
		err = doc.Copy(newpath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *Transaction) Save() error {
	var err error

	if err = t.EnsureDirPath(); err != nil {
		return err
	}

	if err = t.SaveBeancount(); err != nil {
		return err
	}

	if err = t.SaveTxnDocuments(); err != nil {
		return err
	}

	return nil
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

type auxiliary_posting struct {
	Flag     string `json:"flag"`
	Account  string `json:"account"`
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type auxiliary_txn struct {
	Date      string              `json:"date"`
	Flag      string              `json:"flag"`
	Payee     string              `json:"payee"`
	Narration string              `json:"narration"`
	Postings  []auxiliary_posting `json:"postings"`
	Documents []TxnDocument       `json:"documents"`
}

func saveTransactionHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	// Have to decode with auxiliary structs b/c date and numbers come as strings
	// idea is from https://mlafeldt.github.io/blog/decoding-yaml-in-go/
	var aux_txn auxiliary_txn

	if err := decoder.Decode(&aux_txn); err != nil {
		sendError(w, err)
		return
	}

	date, _ := time.Parse("2006-01-02", aux_txn.Date[0:10])

	// Filter out empty docs
	documents := []TxnDocument{}
	for _, doc := range aux_txn.Documents {
		if len(doc.Filename) > 0 {
			documents = append(documents, doc)
		}
	}

	txn := Transaction{
		Date:      date,
		Flag:      aux_txn.Flag,
		Payee:     s.Replace(aux_txn.Payee, `"`, `'`, -1),
		Narration: s.Replace(aux_txn.Narration, `"`, `'`, -1),
		Documents: documents,
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

	if err := txn.Save(); err != nil {
		sendError(w, err)
		return
	}

	enc := json.NewEncoder(w)

	data := make(map[string]interface{})
	data["flash"] = "Saved"

	w.Header().Set("Content-type", "application/json")
	enc.Encode(data)
}

func completionsHandler(w http.ResponseWriter, r *http.Request) {
	globpath := filepath.Join(config.BillsFolder, "*", "*", "*", "*.beancount")
	paths, _ := filepath.Glob(globpath)

	completions := make(map[string][]string)

	completions["payees"] = []string{}
	completions["tags"] = []string{}
	completions["links"] = []string{}

	for _, path := range paths {
		c, _ := ioutil.ReadFile(path)
		text := string(c)
		txn := Transaction{}
		if err := txn.ParseBeancount(text); err != nil {
			log.Printf("%v", err)
		} else {
			if len(txn.Payee) > 0 {
				completions["payees"] = append(completions["payees"], txn.Payee)
			}
			if len(txn.Link) > 0 {
				completions["links"] = append(completions["links"], txn.Link)
			}
			if len(txn.Tags) > 0 {
				for _, t := range txn.Tags {
					completions["tags"] = append(completions["tags"], t)
				}
			}
		}
	}

	completions["payees"] = UniqStr(completions["payees"])
	completions["tags"] = UniqStr(completions["tags"])
	completions["links"] = UniqStr(completions["links"])

	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(completions)
}

func accountsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := ioutil.ReadFile(config.MainBeancountFile)
	if err != nil {
		sendError(w, err)
		return
	}

	data := []string{}
	var matches [][]string

	re := regexp.MustCompile(`\n[^ ]+ +open +([^ \n]+)`)

	matches = re.FindAllStringSubmatch(string(c), -1)

	for _, m := range matches {
		data = append(data, m[1])
	}

	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

func currenciesHandler(w http.ResponseWriter, r *http.Request) {
	c, err := ioutil.ReadFile(config.MainBeancountFile)
	if err != nil {
		sendError(w, err)
		return
	}

	data := []string{}
	var matches [][]string

	re := regexp.MustCompile(`\noption "operating_currency" +"([^ \n]+)"`)

	matches = re.FindAllStringSubmatch(string(c), -1)

	for _, m := range matches {
		data = append(data, m[1])
	}

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
		time.Sleep(time.Duration(3) * time.Second)
	}

	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

func (c conf) updateMainBeancountFile() {
	return
}

func (c conf) startServer() {
	port := c.ServerPort
	router := mux.NewRouter()

	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/save-transaction", saveTransactionHandler).Methods("POST")
	router.HandleFunc("/upload", uploadHandler).Methods("POST")
	router.HandleFunc("/completions.json", completionsHandler).Methods("GET")
	router.HandleFunc("/accounts.json", accountsHandler).Methods("GET")
	router.HandleFunc("/currencies.json", currenciesHandler).Methods("GET")

	n := MyClassic()
	n.UseHandler(router)

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	// Print welcome message
	fmt.Println(figletString("B2B"))
	fmt.Println(fmt.Sprintf("Listening on http://localhost:%d", port))

	// Start the blocking server loop.
	log.Fatal(http.Serve(l, n))
}

func (c conf) openBrowser() {
	if !developmentMode {
		err := open.Start(fmt.Sprintf("http://localhost:%d", c.ServerPort))
		if err != nil {
			log.Println(err)
		}
	}
}

func cleanup() {
	fmt.Println("removing temp folder")
	os.RemoveAll(appTempDir)
}

func main() {
	var err error

	app := cli.NewApp()
	app.Name = "bills-to-beans"
	app.Usage = "bills-to-beans"

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

	app.Action = func(c *cli.Context) {
		// No arguments, so we're a desktop app
		// - updated main beanfile
		// - start a server
		// - open the browser
		if c.NArg() < 1 {
			config.updateMainBeancountFile()
			config.startServer()
			config.openBrowser()
		}
	}

	app.Run(os.Args)
}
