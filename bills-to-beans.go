package main

import (
	"encoding/json"
	//"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/codegangsta/negroni"
	//"github.com/davecgh/go-spew/spew"
	"github.com/getwe/figlet4go"
	"github.com/gorilla/mux"
	"github.com/skratchdot/open-golang/open"
	"log"
	//"math"
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
	Saved   bool      `json:"saved"`
}

type Posting struct {
	Flag     string  `json:"flag"`
	Account  string  `json:"account"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type Transaction struct {
	Date      time.Time  `json:"date"`
	Flag      string     `json:"flag"`
	Payee     string     `json:"payee"`
	Narration string     `json:"narration"`
	Tags      []string   `json:"tags"`
	Link      string     `json:"link"`
	Postings  []Posting  `json:"postings"`
	Documents []Document `json:"documents"`
}

type Balance struct {
	Date          time.Time `json:"date"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	SourceAccount string    `json:"source_account"`
	TargetAccount string    `json:"target_account"`
	Padded        bool      `json:"padded"`
}

func (d Document) String() string {
	return fmt.Sprintf(
		"%s document %s %s",
		d.Date.Format("2006-01-02"),
		d.Account,
		`"`+d.Path+`"`,
	)
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
		out = append(out, fmt.Sprintf("%q", t.Payee))
	}
	if len(t.Narration) > 0 {
		out = append(out, fmt.Sprintf("%q", t.Narration))
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

func (t Transaction) tagFmt() string {
	if len(t.Tags) > 0 {
		return "#" + s.Join(t.Tags, " #")
	}
	return ""
}

func (t Transaction) linkFmt() string {
	if len(t.Link) > 0 {
		return "^" + s.Replace(t.Link, " ", "-", -1)
	}
	return ""
}

func (t Transaction) String() string {
	out := ""

	firstLineParts := []string{
		t.Date.Format("2006-01-02"),
		t.flagFmt(),
		t.titleFmt(),
		t.tagFmt(),
		t.linkFmt(),
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
	if len(t.Postings) == 0 {
		return "0.00 EUR"
	}

	// 1 or 2 postings, first is the amount
	if len(t.Postings) <= 2 {
		return fmt.Sprintf("%.2f %s", t.Postings[0].Amount, t.Postings[0].Currency)
	}
	return ""
}

func sanitizeFilename(text string) string {
	out := regexp.MustCompile(`[^\w\.\'ãÃáÁíÍêÊéÉçÇ-]`).ReplaceAllString(text, " ")
	out = regexp.MustCompile(`  +`).ReplaceAllString(out, " ")
	return out
}

func (t Transaction) sanitizedBase() string {
	return sanitizeFilename(s.Join(
		[]string{
			t.Date.Format("2006-01-02"),
			t.Payee,
			t.Narration,
			t.sumAmountFmt(),
		},
		" _ ",
	))
}

func (t Transaction) DirPath() string {
	return filepath.Join(
		config.BillsFolder,
		fmt.Sprintf("%04d", t.Date.Year()),
		fmt.Sprintf("%02d", t.Date.Month()),
		t.sanitizedBase(),
	)
}

func (t Transaction) BeancountFilename() string {
	return "transaction.beancount"
}

func (t Transaction) SaveBeancount() error {
	var err error

	if err = os.MkdirAll(t.DirPath(), 0755); err != nil {
		return err
	}

	path := filepath.Join(t.DirPath(), t.BeancountFilename())

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

// http://stackoverflow.com/a/21061062/195141
func (d Document) Copy(dst string) error {
	in, err := os.Open(d.Path)
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

func (t Transaction) SaveDocuments() error {
	var err error
	dirPath := t.DirPath()

	for idx, doc := range t.Documents {
		if !doc.Saved {
			newpath := filepath.Join(
				dirPath,
				fmt.Sprintf("doc%d%s", idx+1, filepath.Ext(doc.Path)),
			)
			err = doc.Copy(newpath)
			if err != nil {
				return err
			}
			doc.Path = newpath
			doc.Saved = true
			t.Documents[idx] = doc
		}
	}

	return nil
}

func (t Transaction) Save() error {
	var err error

	if err = t.SaveBeancount(); err != nil {
		return err
	}

	if err = t.SaveDocuments(); err != nil {
		return err
	}

	return nil
}

// TODO
func (t Balance) Save() error {
	return nil
}

// TODO
func (t Document) Save() error {
	return nil
}

// TODO
// return interface slice?
func DirectivesFromBills() []Transaction {
	return nil
}

// TODO
func UpdateMainBeancount() error {
	return nil
}

func MyClassic() *negroni.Negroni {
	return negroni.New(negroni.NewRecovery(), negroni.NewLogger(), negroni.NewStatic(Dir(useLocal, "/public")))
}

func GetLocalIP() string {
	// GetLocalIP returns the non loopback local IP of the host
	// http://stackoverflow.com/a/31551220/195141
	// could also do https://www.socketloop.com/tutorials/golang-how-do-I-get-the-local-ip-non-loopback-address
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
}

func saveTransactionHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	// Have to decode with auxiliary structs b/c date and numbers come as strings
	// idea is from https://mlafeldt.github.io/blog/decoding-yaml-in-go/
	var aux_txn auxiliary_txn

	if err := decoder.Decode(&aux_txn); err != nil {
		log.Printf("%v", err)
		return
	}

	date, _ := time.Parse("2006-01-02", aux_txn.Date[0:10])

	txn := Transaction{
		Date:      date,
		Flag:      aux_txn.Flag,
		Payee:     aux_txn.Payee,
		Narration: aux_txn.Narration,
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
		log.Printf("%v", err)
		return
	}

	data := make(map[string]interface{})
	data["msg"] = txn

	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

func figletString(text string) string {
	ascii := figlet4go.NewAsciiRender()
	renderStr, _ := ascii.Render(text)
	return renderStr
}

func startServer(port int) {
	router := mux.NewRouter()

	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/save-transaction", saveTransactionHandler).Methods("POST")

	n := MyClassic()
	n.UseHandler(router)

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}

	// Open the browser
	if !developmentMode {
		err = open.Start(fmt.Sprintf("http://localhost:%d", port))
		if err != nil {
			log.Println(err)
		}
	}

	// Print welcome message
	fmt.Println(figletString("B2B"))
	fmt.Println(fmt.Sprintf("Listening on http://localhost:%d", port))

	// Start the blocking server loop.
	log.Fatal(http.Serve(l, n))
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
		// No arguments, start a server
		if c.NArg() < 1 {
			startServer(config.ServerPort)
		}
	}

	app.Run(os.Args)
}
