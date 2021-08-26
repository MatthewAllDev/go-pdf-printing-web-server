package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	wkhtml "github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"html/template"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func main() {
	log.SetOutput(os.Stdout)
	if _, err := os.Stat("temp"); err != nil {
		err = os.Mkdir("temp", 0777)
		if err != nil {
			log.Fatal("Error creating the \"temp\" directory.")
		}
	}
	config, err := getConfig()
	if err != nil {
		log.Fatal(fmt.Sprintf("configuration error: %v", err))
	}
	mainHandler := func(w http.ResponseWriter, r *http.Request) {
		err := generateAndPrintFile(r, config)
		if err != nil {
			log.Println(err)
			return
		}
	}
	http.HandleFunc("/", mainHandler)
	log.Fatal(http.ListenAndServe(":" + config.Port, nil))
}

func getConfig() (Config, error) {
	var config Config
	err := os.Setenv("WKHTMLTOPDF_PATH", "bin")
	if err != nil {
		return config, err
	}
	config.PrintOut = true
	jsonData, err := ioutil.ReadFile("config.json")
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		return config, err
	}
	err = config.validate()
	return config, err
}

func generateAndPrintFile(r *http.Request, config Config) error {
	printJob, err := createPrintJob(r, config)
	if err != nil {
		return err
	}
	pdfFilePath := ""
	if !printJob.isFileCreated {
		htmlFilePath, err := createHtmlFile(printJob)
		if !config.DebugMode {
			defer func(name string) {
				_ = os.Remove(name)
			}(htmlFilePath + ".html")
		}
		if err != nil {
			return err
		}
		pdfFilePath, err = convertHtmlToPdf(htmlFilePath, printJob.printer)
		if err != nil {
			return err
		}
	} else {
		pdfFilePath = printJob.filePath
	}
	if !config.DebugMode {
		defer func(name string) {
			_ = os.Remove(name)
		}(pdfFilePath)
	}
	output, err := printPdfFile(pdfFilePath, printJob.printer, config.DebugMode)
	if err != nil {
		log.Print(output)
	}
	return err
}

func createPrintJob(r *http.Request, config Config) (PrintJob, error) {
	var printJob PrintJob
	templateParameters := make(map[string]string)
	err := r.ParseForm()
	if err != nil {
		return printJob, err
	}
	for key, value := range r.Form {
		if len(value) > 1 {
			err = errors.New(fmt.Sprintf("Key \"%s\" in reqeust has more than 1 value.", key))
			return printJob, err
		}
		switch key {
		case "template":
			printJob.templateName = value[0]
			printJob.templatePath = fmt.Sprintf("templates/%s.html", value[0])
		case "printer":
			printJob.printerKey = value[0]
			printJob.printer = config.Printers[value[0]]
		case "file_data":
			filePath := fmt.Sprintf("temp/%v.pdf", time.Now().Unix())
			file, err := os.Create(filePath)
			if err != nil {
				err = errors.New(fmt.Sprintf("Unable to create file from file_data parameter in request: %s", err))
				return printJob, err
			}
			_, err = file.Write([]byte(value[0]))
			if err != nil {
				err = errors.New(fmt.Sprintf("Unable to write file from file_data parameter in request: %s", err))
				return printJob, err
			}
			_ = file.Close()
			printJob.filePath = filePath
			printJob.isFileCreated = true
		default:
			templateParameters[key] = value[0]
		}
	}
	err = printJob.validate()
	if err != nil {
		return printJob, err
	}
	printJob.templateParameters = templateParameters
	return printJob, nil
}

func createHtmlFile(job PrintJob) (string, error) {
	fileName := fmt.Sprintf("temp/%v", time.Now().Unix())
	tmpl, err := template.ParseFiles(job.templatePath)
	if err != nil {
		return "", err
	}
	if value, ok := job.templateParameters["BARCODE_DATA"]; ok {
		barcodeIMG, err := createBarcodePng(value)
		if err != nil {
			return "", err
		}
		imgFileData, err := ioutil.ReadFile(barcodeIMG)
		if err != nil {
			return "", err
		}
		imgFileDataBase64 := base64.StdEncoding.EncodeToString(imgFileData)
		job.templateParameters["BARCODE_IMAGE_BASE64"] = imgFileDataBase64
		_ = os.Remove(barcodeIMG)
	}
	file, err := os.Create(fileName + ".html")
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(file, job.templateParameters)
	if err != nil {
		return "", err
	}
	err = file.Close()
	if err != nil {
		return "", err
	}
	return fileName, err
}

func createBarcodePng(barcodeData string) (string, error) {
	barcode128, err := code128.Encode(barcodeData)
	if err != nil {
		return "", err
	}
	barcode128Scaled, err := barcode.Scale(barcode128, 1000, 1000)
	if err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("temp/%v.png", time.Now().Unix())
	file, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	err = png.Encode(file, barcode128Scaled)
	if err != nil {
		return "", err
	}
	err = file.Close()
	if err != nil {
		return "", err
	}
	return fileName, err
}

func convertHtmlToPdf(htmlPath string, printer Printer) (string, error) {
	var err error = nil
	pdf, err :=  wkhtml.NewPDFGenerator()
	pdf.Dpi.Set(300)
	pdf.PageWidth.Set(printer.PageWidth)
	pdf.PageHeight.Set(printer.PageHeight)
	pdf.MarginTop.Set(0)
	pdf.MarginLeft.Set(0)
	pdf.MarginRight.Set(0)
	pdf.MarginBottom.Set(0)
	if err != nil {
		return "", err
	}
	pdf.AddPage(wkhtml.NewPage(htmlPath + ".html"))
	err = pdf.Create()
	if err != nil {
		return "", err
	}
	err = pdf.WriteFile(htmlPath + ".pdf")
	if err != nil {
		return "", err
	}
	return htmlPath + ".pdf", err
}

func printPdfFile(pdfFilePath string, printer Printer, debugMode bool) (string, error) {
	parameters := make(map[string]string)
	parameters["-sDEVICE"] = "mswinpr2"
	parameters["-dBATCH"] = ""
	parameters["-dNOPAUSE"] = ""
	parameters["-dNOPROMPT"] = ""
	parameters["-dNoCancel"] = ""
	parameters["-dPDFFitPage"] = ""
	parameters["-dNumCopies"] = "1"
	parameters["-empty"] = ""
	parameters["-dPrinted"] = ""
	parameters["-dNOSAFER"] = ""
	parameters["-dDEVICEWIDTHPOINTS"] = fmt.Sprintf("%.0f", float64(printer.PageWidth) / 25.4 * 72)
	parameters["-dDEVICEHEIGHTPOINTS"] = fmt.Sprintf("%.0f", float64(printer.PageHeight) / 25.4 * 72)
	parameters["-sOutputFile"] = "\"%printer%" + fmt.Sprintf("%s\"", printer.Name)
	currentPath, _ := os.Getwd()
	cmd := exec.Command("powershell",
		"/C",
		fmt.Sprintf("%s %s \"%s/%s\"",
			"bin/gswin64c.exe",
			parametersMapToString(parameters),
			currentPath,
			pdfFilePath))
	if debugMode {
		log.Println(cmd)
	}
	output, err := cmd.Output()
	if err != nil {
		return string(output), err
	}
	return string(output), err
}

func parametersMapToString(parameters map[string]string) string {
	parametersString := ""
	for key, value := range parameters {
		if value != "" {
			parametersString += fmt.Sprintf("%s=%s ", key, value)
		} else {
			parametersString += key + " "
		}
	}
	return parametersString
}

type Config struct {
	Port string `json:"port"`
	Printers map[string]Printer `json:"printers"`
	DebugMode bool `json:"debug_mode"`
	PrintOut bool `json:"print_out"`
}

func (config Config) validate() error {
	if config.DebugMode {
		log.Println("WARNING! \"debug_mode\" in config file set \"true\"")
	}
	if !config.PrintOut {
		log.Println("WARNING! \"print_out\" in config file set \"false\"")
	}
	_, err := strconv.Atoi(config.Port)
	if err != nil {
		return errors.New("config validation: port parameter undefined or filled incorrectly")
	}
	if len(config.Printers) == 0 {
		return errors.New("config validation: printers parameter undefined or filled incorrectly")
	} else {
		for printerKey, printer := range config.Printers {
			err = printer.validate()
			if err != nil {
				return errors.New(fmt.Sprintf("config validation: printer \"%s\" is not defined correctly\n\t%v",
					printerKey, err))
			}
		}
	}
	return nil
}

type Printer struct {
	Name      string `json:"name"`
	PageWidth uint    `json:"page_width"`
	PageHeight uint `json:"page_height"`
	DPI uint `json:"dpi"`
}

func (printer Printer) validate() error {
	if len(printer.Name) == 0 {
		return errors.New("printer validation: \"name\" undefined or empty")
	}
	if printer.PageWidth == 0 {
		return errors.New("printer validation: \"page_width\" undefined or zero")
	}
	if printer.PageHeight == 0 {
		return errors.New("printer validation: \"page_height\" undefined or zero")
	}
	if printer.DPI == 0 {
		return errors.New("printer validation: \"dpi\" undefined or zero")
	}
	return nil
}

type PrintJob struct {
	templateName string
	templatePath string
	printerKey string
	printer Printer
	templateParameters map[string]string
	filePath string
	isFileCreated bool
}

func (printJob PrintJob) validate() error {
	if len(printJob.filePath) == 0 {
		if len(printJob.templateName) == 0 {
			return errors.New("print job validation: template or file_data not defined in request")
		}
		if _, err := os.Stat(printJob.templatePath); err != nil {
			return errors.New(fmt.Sprintf("print job validation: template \"%s\" does't exist",
				printJob.templatePath))
		}
	}
	if len(printJob.printerKey) == 0 {
		return errors.New("print job validation: printer not defined in request")
	}
	if err := printJob.printer.validate(); err != nil {
		return errors.New(fmt.Sprintf("print job validation: printer \"%s\" does't exist in config file",
			printJob.printerKey))
	}
	return nil
}