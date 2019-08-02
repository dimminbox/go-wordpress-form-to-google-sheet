package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	sheets "google.golang.org/api/sheets/v4"
)

// GoogleSheet - структура для работы с гугл таблицами
type GoogleSheet struct {
	Spreadsheet string
	Service     *sheets.Service
}

/*func (g *GoogleSheet) Write() (result Result) {
	row := 1
	column := 2

	g.Sheet.Update(row, column, "22222")
	g.Sheet.Update(3, 2, "33333")

	err := g.Sheet.Synchronize()
	if err != nil {
		result = GenError(err.Error())
	}

	result.Result = true
	return
}*/

// Init соединяется с Google
func (g *GoogleSheet) Init() (result Result) {

	var err error
	data, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		result = GenError(err.Error())
		return
	}

	config, err := google.ConfigFromJSON(data, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := g.getClient(config)

	g.Service, err = sheets.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	} else {
		result.Result = true
	}

	/*readRange := "Response!A2:E"
	_, err = g.Service.Spreadsheets.Values.Get(g.Spreadsheet, readRange).Do()

	if err != nil {
		result = GenError(err.Error())
		return
	} else {
		result.Result = true
	}*/

	return
}

// Keys получает все ключи записей
func (g *GoogleSheet) Keys() (keys map[int]string, result Result) {

	keys = map[int]string{}

	readRange := "Response!A1:A1000000"
	resp, err := g.Service.Spreadsheets.Values.Get(g.Spreadsheet, readRange).Do()

	if err != nil {
		result = GenError(err.Error())
	} else {

		for i, val := range resp.Values {
			if i > 1 {
				keys[i] = fmt.Sprintf("%s", val[0])
			}
		}
		result.Result = true
	}

	return
}

// GetColumnNames возвращает название ячеек в Google Sheet
func GetColumnNames(index int) (result []string) {

	var alphabet = [...]string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M",
		"N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}

	for _, liter := range alphabet {
		result = append(result, fmt.Sprintf("%s%d", liter, index))
	}

	for _, liter1 := range alphabet {
		for _, liter2 := range alphabet {
			result = append(result, fmt.Sprintf("%s%s%d", liter1, liter2, index))
		}
	}

	return

}

// Columns получает все колонки вопросов
func (g *GoogleSheet) Columns() (columns map[string]map[string]string, result Result) {

	header := GetColumnNames(1)
	answers := GetColumnNames(2)

	columns = map[string]map[string]string{}

	readRange := "Response!A2:ZZ2"
	resp, err := g.Service.Spreadsheets.Values.Get(g.Spreadsheet, readRange).Do()
	//fmt.Printf("%+v\n", resp)

	if err != nil {
		result = GenError(err.Error())
	} else {

		var responseName string
		for index, val := range resp.Values[0] {

			if val == "" {
				continue
			}
			if len(answers) <= (index + 1) {
				break
			}

			// проверяем название анкеты
			var readRange string

			readRange = fmt.Sprintf("Response!%s:%s", header[index], header[index])
			resp, err := g.Service.Spreadsheets.Values.Get(g.Spreadsheet, readRange).Do()
			if err == nil {

				if len(resp.Values) == 1 {
					if len(resp.Values[0]) == 1 {
						responseName = fmt.Sprintf("%s", resp.Values[0][0])
						columns[responseName] = map[string]string{}
					} else {
						result = GenError(fmt.Sprintf("Не стандартный ответ длины %d response - %+v", len(resp.Values[0]), resp.Values[0]))
						return
					}
				}

			} else {
				result = GenError(err.Error())
				return
			}

			if responseName == "" {
				result = GenError("Не удаётся найти название анкеты.")
				return
			}

			// заполняем названия вопросов
			/*readRange = fmt.Sprintf("Response!%s:%s", answers[index], answers[index])
			resp, err = g.Service.Spreadsheets.Values.Get(g.Spreadsheet, readRange).Do()
			if err == nil && len(resp.Values) == 1 {

				if len(resp.Values[0]) == 1 {
					columns[responseName][answers[index]] = fmt.Sprintf("%s", resp.Values[0][0])
				} else {
					result = GenError(fmt.Sprintf("Не стандартный ответ длины %d response - %+v", len(resp.Values[0]), resp.Values[0]))
				}

			} else {
				result = GenError(fmt.Sprintf("err - %s, response - %+v", err.Error(), resp.Values))
				return
			}*/
			columns[responseName][answers[index]] = fmt.Sprintf("%s", val)

		}

		result.Result = true
	}

	return
}

// Retrieve a token, saves the token, then returns the generated client.
func (g *GoogleSheet) getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := g.tokenFromFile(tokFile)
	if err != nil {
		tok = g.getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func (g *GoogleSheet) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func (g *GoogleSheet) tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
