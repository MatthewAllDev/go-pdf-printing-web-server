Go PDF printing web server
========================
Dependencies 
-------------------------
+ [go-wkhtmltopdf](https://github.com/SebastiaanKlippert/go-wkhtmltopdf "go-wkhtmltopdf") - [License "MIT"](https://mit-license.org/ "License \"MIT\"")
+ and [wkhtmltopdf](https://github.com/wkhtmltopdf/wkhtmltopdf "wkhtmltopdf") (binary file in "bin" directory) - [License "LGPL-3.0"](https://www.gnu.org/licenses/lgpl-3.0.html "License \"LGPL-3.0\"")
+ [barcode](https://github.com/boombuler/barcode "barcode") - [License "MIT"](https://mit-license.org/ "License \"MIT\"")
+ [Ghostscript](https://www.ghostscript.com "Ghostscript") (binary file in "bin" directory) - [License "AGPL"](https://www.gnu.org/licenses/agpl-3.0.html "License \"AGPL\"")

Request:
-------------------------
+ ***file_data*** - *string* - binary data of the file in base64
+ ***template*** - *string* - template file name in directory "templates"
+ ***printer*** - *string* - the key of the printer configured in the configuration file
+ ***template parameters*** - *string* - keys must match the parameters in the template file
*\* If the "file_data" query parameter is filled, the "template" and template parameters are ignored.*

Template:
-------------------------
If template contains "{{ .BARCODE_DATA}}" server create barcode image and puts bass64 data in
"{{ .BARCODE_IMAGE_BASE64}}"

Config:
-------------------------
+ ***debug_mode*** - *bool* - true value disable the deletion of temp files
+ ***print_out*** - *bool* - false value disable printing
+ ***port*** - *string* - web server port number
+ ***printers*** - *object* - objects of type printer with keys
    + ***"printer_key"*** - *object* - object of type printer with fields:
        + ***name*** - *string* - printer name in OS
        + ***page_width*** - *int* - width of paper
        + ***page_height*** - *int* - height of paper
        + ***dpi*** - *int* - printer dpi, used for create PDF file from template 