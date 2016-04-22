package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/codegangsta/negroni"
	//"github.com/fatih/color"
	"github.com/getwe/figlet4go"
	"github.com/skratchdot/open-golang/open"
	"log"
	"os/signal"
	"syscall"
	//"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/jung-kurt/gofpdf"
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
var serverPort int

var appTempDir string

type Transaction struct {
	Title         string    `json:"title"`
	Date          time.Time `json:"date"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	SourceAccount string    `json:"source-account"`
	TargetAccount string    `json:"target-account"`
	Errors        []error   `json:"-"`
}

type Config struct {
	SaveBillPath string
}

type Image struct {
	Path string `json:"path"`
}

type Bill struct {
	Images      []Image     `json:"images"`
	Transaction Transaction `json:"transaction"`
}

var currentBill Bill

func (t Transaction) String() string {
	if len(t.Errors) > 0 {
		return ""
	}
	fmtStr := `%s * "%s"
  %s %.2f %s
  %s`
	return fmt.Sprintf(
		fmtStr,
		t.Date.Format("2006-01-02"),
		t.Title,
		t.SourceAccount,
		t.Amount,
		t.Currency,
		t.TargetAccount,
	)
}

func (t Transaction) BeanString() string {
	return t.String()
}

func (t *Transaction) parseFilename(name string) (e error) {
	if s.Contains(name, string(filepath.Separator)) {
		name = filepath.Base(name)
	}

	name = s.TrimSuffix(name, filepath.Ext(name))
	name = s.Trim(name, " _-")

	var m string

	// Take the date from the beginning
	dateRe := regexp.MustCompile(`^[0-9-]+`)
	m = dateRe.FindString(name)
	name = s.TrimPrefix(name, m)

	if len(m) == 0 {
		return errors.New(fmt.Sprintf("Missing date\n"))
	}

	// 20151210
	m = s.Replace(m, "-", "", -1)
	if len(m) != 8 {
		return errors.New(fmt.Sprintf("Malformed date: %s\n", m))
	}
	d, err := time.Parse("20060102", m)
	if err != nil {
		return errors.New(fmt.Sprintf("Can't parse date: %s\n", m))
	}
	t.Date = d

	// "120.00 EUR"
	// "120EUR"

	var matches []string

	if len(t.Currency) == 0 {
		currencyRe := regexp.MustCompile(` +([0-9\.]+) *(EUR|GBP|USD)$`)
		matches = currencyRe.FindStringSubmatch(name)
		if len(matches) > 0 {
			a, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				return errors.New(fmt.Sprintf("Can't parse amount: %s\n", m))
			}
			t.Amount = a
			t.Currency = matches[2]
			name = s.TrimSuffix(name, matches[0])
		}
	}

	// "120.29€"
	// "120.29€"
	// "€120.29"
	// "€120.29"
	// "120€29"
	// NOT "120€290"

	if len(t.Currency) == 0 {
		currencyRe := regexp.MustCompile(` +([0-9\.€£$]+)$`)
		matches = currencyRe.FindStringSubmatch(name)

		if len(matches) == 2 {
			if ok, _ := regexp.MatchString(`[€£$]`, matches[1]); !ok {
				return errors.New(fmt.Sprintf("Missing currency: %s\n", matches[1]))
			}

			astr := matches[1]
			switch {
			case s.Contains(astr, "€"):
				t.Currency = "EUR"
			case s.Contains(astr, "£"):
				t.Currency = "GBP"
			case s.Contains(astr, "$"):
				t.Currency = "USD"
			}

			astr = s.Trim(astr, "€£$")
			astr = s.Replace(astr, "€", ".", -1)
			astr = s.Replace(astr, "£", ".", -1)
			astr = s.Replace(astr, "$", ".", -1)

			// More than two digits after the decimal is not accepted
			if nono, _ := regexp.MatchString(`\.[0-9]{3,}$`, astr); nono {
				return errors.New(fmt.Sprintf("Can't parse amount: %s\n", astr))
			}
			a, err := strconv.ParseFloat(astr, 64)
			if err != nil {
				return errors.New(fmt.Sprintf("Can't parse amount: %s\n", astr))
			}

			t.Amount = a

			name = s.TrimSuffix(name, matches[0])
		}
	}

	// If there is still no currency, it is not an amount
	if len(t.Currency) == 0 {
		return errors.New(fmt.Sprintf("Can't parse amount: %s\n", name))
	}

	// What's left must be the title
	t.Title = regexp.MustCompile(`  +`).ReplaceAllString(name, " ")
	t.Title = s.Trim(t.Title, " -_")

	return nil
}

func (t *Transaction) ParsePath(path string) (e error) {
	if err := t.parseFilename(filepath.Base(path)); err != nil {
		t.Errors = append(t.Errors, err)
		return err
	}

	text := s.ToLower(path)

	if s.Contains(text, "tag:exclude") {
		return errors.New("skipping tag:exclude")
	}
	if s.Contains(text, "global") {
		return errors.New("skipping global position")
	}

	switch {
	case s.Contains(text, "donativo"):
		t.SourceAccount = "Assets:Bank:Checking"
		t.TargetAccount = "Assets:General:Donations"

	case s.Contains(text, "petty cash"):
		t.SourceAccount = "Assets:Petty-Cash"
	}

	if len(t.TargetAccount) == 0 {
		var acc string
		switch {

		// Wood
		case s.Contains(text, "lenha"), s.Contains(text, "wood"):
			acc = "Expenses:Wood"

			// Be Water
		case s.Contains(text, "be water"),
			s.Contains(text, "bewater"):
			acc = "Expenses:Water:Be-Water"

		// Car
		case
			s.Contains(text, "petrol"),
			regexp.MustCompile(`gasol[ií]n`).MatchString(text):
			acc = "Expenses:Car:Gasoline"

		case s.Contains(text, "via verde"),
			s.Contains(text, "viaverde"):
			acc = "Expenses:Car:Via-Verde"

		case s.Contains(text, "carro"):
			acc = "Expenses:Car"

			// Insurances
		case s.Contains(text, "travel insurance"):
			acc = "Expenses:Insurance:Travel"

		case s.Contains(text, "insurance"):
			acc = "Expenses:Insurance"

		// Post CTT
		case s.Contains(text, "ctt"):
			acc = "Expenses:Post:CTT"

		// Phone e Internet
		case
			s.Contains(text, "vodafone"),
			s.Contains(text, "telephone"),
			s.Contains(text, "internet"):
			acc = "Expenses:Phone-Internet:Vodafone"

		// Gas
		case s.Contains(text, "gas coprel"):
			acc = "Expenses:Gas:Coprel"

		case s.Contains(text, "gás"):
			acc = "Expenses:Gas"

		// Tranquilidade
		case s.Contains(text, "tranquilidade"):
			acc = "Expenses:Tranquilidade"

		// Capital investido
		case s.Contains(text, "capital investido"),
			s.Contains(text, "capital invesido"):
			acc = "Expenses:Investment"

			// Forest works
		case regexp.MustCompile(`em[ií]lio`).MatchString(text):
			acc = "Expenses:Forest:SrEmilio"
		case s.Contains(text, "bruno"):
			acc = "Expenses:Forest:SrBruno"
		case s.Contains(text, "tito"):
			acc = "Expenses:Forest:SrTito"

			// Construction
		case s.Contains(text, "engineer"),
			s.Contains(text, "rui"):
			acc = "Expenses:Construction:SrRui"

			// Purchases in Shops, AKI, IKEA, Makro, etc.
		case s.Contains(text, "aki"):
			acc = "Expenses:Purchases:AKI"
		case s.Contains(text, "ikea"):
			acc = "Expenses:Purchases:IKEA"
		case s.Contains(text, "makro"):
			acc = "Expenses:Purchases:Makro"
		case s.Contains(text, "asb"):
			acc = "Expenses:Purchases:Makro"

		}

		t.TargetAccount = acc
	}

	// If nothing was identified, then it is a general expense

	if len(t.SourceAccount) == 0 {
		t.SourceAccount = "Assets:Bank:Checking"
	}
	if len(t.TargetAccount) == 0 {
		t.TargetAccount = "Expenses:General"
	}

	// Expense must be negative
	if s.Split(t.TargetAccount, ":")[0] == "Expenses" && t.Amount > 0 {
		t.Amount = t.Amount * -1
	}

	return nil
}

func pathToTransaction(path string, info os.FileInfo, err error) (e error) {
	if info.IsDir() {
		return nil
	}

	trans := Transaction{}
	if err := trans.ParsePath(path); err != nil {
		fmt.Printf("; %s ; SKIPPING: %s\n", path, err.Error())
		// nil will continue the walk
		return nil
	}

	fmt.Printf("; %s\n\n", path)
	fmt.Printf("%s\n\n", trans.String())

	return nil
}

func MyClassic() *negroni.Negroni {
	return negroni.New(negroni.NewRecovery(), negroni.NewLogger(), negroni.NewStatic(Dir(useLocal, "/public")))
}

func createBeanText(beantext string, path string) error {
	var err error

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(beantext)
	if err != nil {
		return err
	}

	return nil
}

func createPDF(title string, imagePaths []string, beanText string, pdfPath string) (pdf *gofpdf.Fpdf, e error) {

	pdf = gofpdf.New("P", "mm", "A4", "")

	pdf.SetTitle(title, false)

	pdf.SetFont("Helvetica", "", 12)

	pdf.SetHeaderFunc(func() {
		pdf.SetY(5)
		pdf.CellFormat(
			0, 10,
			fmt.Sprintf("%s - Page %d/{nb}", title, pdf.PageNo()),
			"", 0, "C", false, 0, "",
		)
		pdf.Ln(20)
	})

	options := gofpdf.ImageOptions{
		ReadDpi:   false,
		ImageType: "", // infers from extension
	}

	// TODO Also see ExampleFpdf_CreateTemplate()

	for _, path := range imagePaths {
		pdf.AddPage()
		pdf.ImageOptions(path, 0, 15, 210, 0, false, options, 0, "")
	}

	pdf.AddPage()
	pdf.SetFont("Courier", "", 12)

	for _, txt := range s.Split(beanText, "\n") {
		pdf.CellFormat(0, 10, txt, "", 1, "LT", false, 0, "")
	}

	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return nil, err
	}

	return pdf, nil
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
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]string)
	data["localAddress"] = fmt.Sprintf("http://%s:%d", GetLocalIP(), serverPort)

	t, _ := template.New("index").Parse(FSMustString(useLocal, "/public/index.html.tmpl"))
	t.Execute(w, data)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	r.ParseMultipartForm(32 << 20) // using 32 MB memory

	file, handler, err := r.FormFile("file")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer file.Close()

	f, err := ioutil.TempFile(appTempDir, "bill_")
	if err != nil {
		fmt.Println(err)
		return
	}

	p, _ := filepath.Abs(f.Name())
	f.Close()

	ct := handler.Header.Get("Content-Type")

	if !(ct == "image/png" || ct == "image/jpeg") {
		err = errors.New("must be png or jpeg")
		fmt.Println(err)
		return
	}

	ext := "." + s.Split(ct, "/")[1]
	path := p + ext

	if err = os.Rename(p, path); err != nil {
		fmt.Println(err)
		return
	}

	f, err = os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	if _, err = io.Copy(f, file); err != nil {
		fmt.Println(err)
		return
	}

	data := make(map[string]interface{})
	info, _ := f.Stat()
	data["path"] = path
	data["size"] = info.Size()

	// Simulate waiting time for upload during development
	if developmentMode {
		time.Sleep(time.Duration(3) * time.Second)
	}

	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

// TODO
func readConfig() Config {
	return Config{SaveBillPath: "../New Bills"}
}

type SaveTransaction struct {
	Title         string `json:"title"`
	Date          string `json:"date"`
	Amount        string `json:"amount"`
	Currency      string `json:"currency"`
	SourceAccount string `json:"source-account"`
	TargetAccount string `json:"target-account"`
}

type SaveBill struct {
	Images      []Image         `json:"images"`
	Transaction SaveTransaction `json:"transaction"`
}

func (b *SaveBill) createFiles() error {
	config := readConfig()

	var beantext, basename, path, ext string
	var err error

	basename = fmt.Sprintf("%s - %s - %s %s", b.Transaction.Date, b.Transaction.Title, b.Transaction.Amount, b.Transaction.Currency)
	path = filepath.Join(config.SaveBillPath, basename)

	beantext = `Beantext
with
linebreaks`

	var imagePaths []string
	for _, img := range b.Images {
		if len(img.Path) > 0 {
			imagePaths = append(imagePaths, img.Path)
		}
	}

	ext = ".pdf"
	if _, err = createPDF(b.Transaction.Title, imagePaths, beantext, path+ext); err != nil {
		fmt.Println(err)
		return nil
	}

	ext = ".bean"
	if err = createBeanText(beantext, path+ext); err != nil {
		fmt.Println(err)
		return nil
	}

	return nil
}

func doneHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	var bill SaveBill
	if err := decoder.Decode(&bill); err != nil {
		fmt.Println(err)
		return
	}

	if err := bill.createFiles(); err != nil {
		fmt.Println(err)
		return
	}

	data := make(map[string]interface{})
	data["msg"] = bill

	w.Header().Set("Content-type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(data)
}

func startServer(port int) {
	router := mux.NewRouter()

	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/upload", uploadHandler).Methods("POST")
	router.HandleFunc("/done", doneHandler).Methods("POST")

	n := MyClassic()
	n.UseHandler(router)

	// http://stackoverflow.com/a/32742904/195141
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", serverPort))
	if err != nil {
		log.Fatal(err)
	}

	if !developmentMode {
		// The browser can connect now because the listening socket is open.
		err = open.Start(fmt.Sprintf("http://localhost:%d", serverPort))
		if err != nil {
			log.Println(err)
		}
	}

	str := "B2B"

	ascii := figlet4go.NewAsciiRender()
	renderStr, _ := ascii.Render(str)
	fmt.Println(renderStr)

	fmt.Println(fmt.Sprintf("Listening on http://localhost:%d", serverPort))

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
	app.Usage = "bills-to-beans [folder]"

	serverPort = 3030

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
			startServer(serverPort)
		}

		dirname := c.Args()[0]
		// check if exists without opening
		d, err := os.Open(dirname)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		defer d.Close()

		filepath.Walk(dirname, pathToTransaction)

	}

	app.Run(os.Args)
}
