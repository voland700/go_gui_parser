package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	_ "fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
	"github.com/xuri/excelize/v2"
)

func main() {
	myApp := app.New()
	myApp.Settings().SetTheme(theme.DarkTheme())
	colorTheme := true

	myWindow := myApp.NewWindow("Парсер контента")
	myWindow.Resize(fyne.NewSize(800, 545))
	icon, _ := fyne.LoadResourceFromPath("icon.png")
	myWindow.SetIcon(icon)

	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	label := canvas.NewText("Cылки для парсинга", color.RGBA{85, 85, 85, 255})
	textarea := widget.NewMultiLineEntry()
	textarea.SetMinRowsVisible(25)

	spacerTop := canvas.NewText("spacer", color.Transparent)

	themeChange := widget.NewButtonWithIcon(" Theme ", theme.ViewRefreshIcon(), func() {
		colorTheme = !colorTheme
		if colorTheme {
			myApp.Settings().SetTheme(theme.DarkTheme())
		} else {
			myApp.Settings().SetTheme(theme.LightTheme())
		}
	})

	btnStart := widget.NewButtonWithIcon(" Start ", theme.DownloadIcon(), func() {

		conection := checkConnection()
		if !conection {
			dialog.NewInformation("Нет доступа", "Сайт везувий недоступен!", myWindow).Show()
			return
		}

		currentText := textarea.Text
		urls := getLinks(currentText)
		if len(urls) == 0 {
			dialog.NewInformation("Нет ссылок", "Не получены ссылки для парсинга!", myWindow).Show()
			return
		}

		doTheJob := func(urls []string) {
			qty, file_name := parsing(urls) // функция  - парсинг товаров по ссылкам
			if qty > 0 {
				dialog.NewInformation("Парсинг окончен!", fmt.Sprintf("Получено %d товаров, файл: %v", qty, file_name), myWindow).Show()
			} else {
				dialog.NewInformation("Данные не получены", "Что-то пошло не так..", myWindow).Show()
			}
			// Обновление виджетов разрешено делать только из главной горутины,
			// поэтому нужно шедулить обновление виджетов через fyne.Do
			fyne.Do(func() {
				textarea.SetText("")
				progress.Stop()
				progress.Hide()
			})

		}

		//  Старт парсинга после подтверждения в диалоговом окне.
		dialog.ShowCustomConfirm(
			"Приступить к сбору данных?",
			"Начать парсинг",
			"Отказаться",
			widget.NewLabel(fmt.Sprintf("Получено %d ссылок для парсинга", len(urls))),
			func(yes bool) {
				if yes {
					progress.Show()
					progress.Start()
					go doTheJob(urls)
				} else {
					return
				}
			},
			myWindow,
		)

	})

	btnClean := widget.NewButtonWithIcon(" Clean ", theme.DeleteIcon(), func() {
		currentText := textarea.Text
		if currentText == "" {
			return
		} else {
			textarea.SetText("")
		}

	})

	middle := container.New(layout.NewVBoxLayout(), label, textarea)

	right := container.NewVBox(spacerTop, btnStart, btnClean, layout.NewSpacer(), themeChange, spacerTop)

	content := container.NewBorder(progress, nil, nil, right, middle)
	myWindow.SetContent(content)
	myWindow.ShowAndRun()
}

func checkConnection() bool {
	_, err := http.Get("https://vezuviy.su")
	if err != nil {
		return false
	}
	return true
}

func getLinks(currentText string) []string {
	textLines := strings.Split(currentText, "\n")
	urls := []string{}
	if len(textLines) > 0 {
		for _, urlLine := range textLines {
			if urlLine != "" {
				urlLine = strings.TrimSpace(urlLine)
				_, err := url.ParseRequestURI(urlLine)
				if err == nil {
					urls = append(urls, urlLine)
				}
			}

		}

	}
	return urls
}

type products []product
type unique map[int]string

type feature struct {
	name  string
	value string
}
type features []feature
type product struct {
	url            string
	name           string
	sku            string
	price          string
	category       string
	description    string
	main           string
	more           string
	jsonProperties string
	properties     features
}

func getProduct(url string) product {
	doc, _ := getDocument(url)
	images := getImages(doc)
	features := getFeatures(doc)
	product := product{
		url:            url,
		name:           doc.Find("h1").Text(),
		sku:            doc.Find("span.ty-control-group__item").Text(),
		price:          getPrice(doc),
		category:       getCategory(doc),
		description:    getDescription(doc),
		main:           images[0],
		more:           images[1],
		jsonProperties: getJsonFeatures(features),
		properties:     features,
	}
	return product
}

func getDocument(url string) (*goquery.Document, error) {
	response, err := http.Get(url)
	if err != nil {
		log.Fatal("Error while fetching the URL:", err)
	}
	defer response.Body.Close()
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error while reading the response body:", err)
	}
	return doc, nil
}

func getPrice(doc *goquery.Document) string {
	var price string = "0"
	priceText := doc.Find("span.ty-price-num").First().Text()
	digits := []rune{}
	for _, char := range priceText {
		if unicode.IsDigit(char) {
			digits = append(digits, char)
		}
	}
	if len(digits) > 0 {
		price = string(digits)
	}
	return price
}

func getCategory(doc *goquery.Document) string {
	var category string
	category = doc.Find("a.ty-breadcrumbs__a").Last().Text()
	return category
}

func getDescription(doc *goquery.Document) string {
	var description string
	dirtyText := doc.Find("div#content_description")
	if dirtyText.Length() != 0 {
		textToRemove := "Внешний вид и комплектация товара может незначительно отличаться от фотографий на сайте"
		dirtyText.Find("*").Each(func(i int, s *goquery.Selection) {
			elementText := s.Text()
			if strings.Contains(elementText, textToRemove) {
				s.Remove()
			}
		})
		dirtyText.Find("p").Each(func(i int, s *goquery.Selection) {
			elementText := s.Text()
			if elementText == "" {
				s.Remove()
			}
		})
		dirtyHTML, _ := dirtyText.Html()
		p := bluemonday.NewPolicy()
		p.AllowElements("p", "ul", "li", "ol", "br", "b", "table", "tbody", "tr", "td", "th", "h1", "h2", "h3", "h4", "h5", "h6", "span")
		html := p.Sanitize(dirtyHTML)
		description = strings.TrimSpace(html)
	}
	return description
}

func getFeatures(doc *goquery.Document) features {
	var features []feature
	featuresDOM := doc.Find("div#content_features")
	if featuresDOM.Length() != 0 {
		featuresDOM.Find("div.ty-product-feature").Each(func(i int, row *goquery.Selection) {
			var item feature
			item.name = row.Find("div.ty-product-feature__label").Text()
			item.value = row.Find("div.ty-product-feature__value").Text()
			features = append(features, item)
		})
	}
	return features
}

func getJsonFeatures(features features) string {
	type property map[string]string
	properties := []property{}
	if len(features) > 0 {
		for _, feature := range features {
			property := make(map[string]string)
			property["name"] = feature.name
			property["value"] = feature.value
			properties = append(properties, property)
		}
	}
	dataJSON, err := json.Marshal(properties)
	if err != nil {
		fmt.Println("Ошибка кодирования в JSON:", err)
		return ""
	}
	return string(dataJSON)
}

type images [2]string

func getImages(doc *goquery.Document) images {
	images := images{}
	var moreArray []string
	doc.Find("a.cm-image-previewer").Each(func(i int, item *goquery.Selection) {
		if i == 0 {
			url, ok := item.Attr("href")
			if ok {
				images[0] = downloadFile(url, "main")
			}
		} else {
			url, ok := item.Attr("href")
			if ok {
				var moreItem string
				moreItem = downloadFile(url, "more")
				moreArray = append(moreArray, moreItem)
			}
		}
	})
	if len(moreArray) > 0 {
		images[1] = strings.Join(moreArray, ",")
	}
	return images
}

func downloadFile(url string, folder string) string {
	if folder == "" {
		folder = "images"
	}
	dot := strings.LastIndex(url, ".")

	ext := url[dot:]
	path := "upload/" + folder
	filepath := path + "/" + folder + "_" + randomString(10) + ext

	err := os.MkdirAll(path, 0755)
	if err != nil {
		fmt.Println(err)
	}

	out, err := os.Create(filepath)
	if err != nil {
		fmt.Println(err)
		//return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		//return err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Println(err)
		//return err
	}
	//fmt.Println(filepath)
	return filepath
}

func randomString(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	sb := strings.Builder{}
	sb.Grow(n)
	for i := 0; i < n; i++ {
		sb.WriteByte(charset[rand.Intn(len(charset))])
	}
	return sb.String()
}

// -- Удалить
func readFileSource(fileSource string) []string {
	var urls []string
	file, err := os.ReadFile(fileSource)
	if err != nil {
		fmt.Println("Ошибка чтения файла:", err)
		return nil
	}
	urlLines := strings.Split(string(file), "\n")
	if len(urlLines) != 0 {
		for _, urlLine := range urlLines {
			if urlLine != "" {
				urls = append(urls, strings.TrimSpace(urlLine))
			}
		}
	}
	return urls
}

func getUniquePropertyNames(products products) unique {
	var unique = unique{}
	key := 10
	for _, product := range products {
		if len(product.properties) > 0 {
			for _, property := range product.properties {
				if len(unique) == 0 {
					unique[key] = property.name
					key++
				} else {
					is := isUnique(property.name, unique)
					if !is {
						unique[key] = property.name
						key++
					}
				}
			}
		}
	}

	println(unique[key])

	return unique
}

// Проверка наличия уникальных названий характеристик товаров в map unique
func isUnique(propName string, unique map[int]string) bool {
	var found bool = false
	for _, item := range unique {
		if item == propName {
			found = true
			break
		}
	}
	return found
}

func createExcelFile(products products) string {
	unique := getUniquePropertyNames(products)

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	index, err := f.NewSheet("Products")
	if err != nil {
		fmt.Println(err)
		return ""
	}

	f.SetCellValue("Products", "A1", "URL")
	f.SetCellValue("Products", "B1", "NAME")
	f.SetCellValue("Products", "C1", "SKU")
	f.SetCellValue("Products", "D1", "PRICE")
	f.SetCellValue("Products", "E1", "CATEGORY")
	f.SetCellValue("Products", "F1", "DESCRIPTION")
	f.SetCellValue("Products", "G1", "MAIN")
	f.SetCellValue("Products", "H1", "MORE")
	f.SetCellValue("Products", "I1", "JSON_PROPERTIES")

	for i, uniqueName := range unique {
		cell, _ := excelize.CoordinatesToCellName(i, 1)
		f.SetCellValue("Products", cell, uniqueName)
	}

	row := 2
	for _, product := range products {
		strRow := strconv.Itoa(row)
		f.SetCellValue("Products", "A"+strRow, product.url)
		f.SetCellValue("Products", "B"+strRow, product.name)
		f.SetCellValue("Products", "C"+strRow, product.sku)
		f.SetCellValue("Products", "D"+strRow, product.price)
		f.SetCellValue("Products", "E"+strRow, product.category)
		f.SetCellValue("Products", "F"+strRow, product.description)
		f.SetCellValue("Products", "G"+strRow, product.main)
		f.SetCellValue("Products", "H"+strRow, product.more)
		f.SetCellValue("Products", "I"+strRow, product.jsonProperties)
		if len(product.properties) > 0 {
			for _, property := range product.properties {
				for k, propertyName := range unique {
					if propertyName == property.name {
						cell, _ := excelize.CoordinatesToCellName(k, row)
						f.SetCellValue("Products", cell, property.value)
					}

				}

			}

		}
		row++
	}
	f.SetActiveSheet(index)
	file_name := "products_" + randomString(5) + ".xlsx"

	if err := f.SaveAs(file_name); err != nil {
		fmt.Println(err)
		return ""
	}
	return file_name
}

func parsing(urls []string) (int, string) {
	var file_name string
	var qty int
	data := make([]product, 0, len(urls))

	for _, url := range urls {
		product := getProduct(url)
		if product.name != "" {
			data = append(data, product)
		}
	}
	if len(data) > 0 {
		file_name = createExcelFile(data)
		qty = len(data)
	}
	return qty, file_name
}
