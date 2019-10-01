package model

import (
	"context"
	"crypto/md5"
	"encoding/hex"
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
	SheetID     int64
}

// GoogleAction структура для хранения действия по Google таблице
type GoogleAction struct {
	Action      string //insert, delete
	Type        string //column, group
	NameColumn  string //название колонки
	NameGroup   string //название группы
	IndexDelete int64  //номер колонки для удаления
	IndexStart  int64  //номер колонки относительно которой нужно вставить данные
}

// GoogleRow структура для храннения данных в строке
type GoogleRow struct {
	Index    string
	Checksum string
	Data     []AnswerOption
}

const workTab = "Response"

// Init соединяется с Google
func (g *GoogleSheet) Init() (result Result) {

	fmt.Println("Google Init")

	var err error
	data, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		result = GenError(err.Error())
		return
	}

	config, err := google.ConfigFromJSON(data, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
		return
	}

	client := g.getClient(config)

	g.Service, err = sheets.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
		return
	}

	result = g.sheetSet()
	if result.Result {
		if g.SheetID == 0 {
			result = g.SheetAdd()
		}
	}

	return
}

// Data получает все ключи записей
func (g *GoogleSheet) Data() (rows []GoogleRow, result Result) {

	fmt.Println("Google Data")
	readRange := fmt.Sprintf("%s!A1:ZZ1000000", workTab)
	resp, err := g.Service.Spreadsheets.Values.Get(g.Spreadsheet, readRange).Do()

	if err != nil {
		result = GenError(err.Error())
	} else {

		for i, vals := range resp.Values {
			var items string
			if i > 1 {
				for _, item := range vals {
					items = fmt.Sprintf("%s%s", items, fmt.Sprintf("%s", item))
				}

				hasher := md5.New()
				hasher.Write([]byte(items))

				rows = append(rows, GoogleRow{
					Index:    fmt.Sprintf("%s", vals[0]),
					Checksum: hex.EncodeToString(hasher.Sum(nil)),
				})
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
func (g *GoogleSheet) Columns() (columns map[string][]string, result Result) {

	fmt.Println("Google Columns")

	header := GetColumnNames(1)
	answers := GetColumnNames(1)

	columns = map[string][]string{}

	readRange := "Response!A2:ZZ2"
	resp, err := g.Service.Spreadsheets.Values.Get(g.Spreadsheet, readRange).Do()

	if err != nil {
		result = GenError(err.Error())
	} else {

		var responseName string

		if len(resp.Values) == 1 {
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
							columns[responseName] = []string{}
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
				} else {
					columns[responseName] = append(columns[responseName], fmt.Sprintf("%s", val))
				}

			}
		}

		result.Result = true
	}

	return
}

// sheetSet устанавливает ID вкладки по названию
func (g *GoogleSheet) sheetSet() (result Result) {

	var err error
	resp, err := g.Service.Spreadsheets.Get(g.Spreadsheet).Do()
	if err != nil {
		result = GenError(err.Error())
		return
	}

	for _, sheet := range resp.Sheets {
		if sheet.Properties.Title == workTab {
			g.SheetID = sheet.Properties.SheetId
			result.Result = true
			return
		}
	}

	return
}

// SheetRemove удаляет вкладку
func (g *GoogleSheet) SheetRemove() (result Result) {

	var req sheets.Request
	var buRequest *sheets.BatchUpdateSpreadsheetRequest
	var err error

	if g.SheetID == 0 {
		result.Result = true
		return
	}

	req = sheets.Request{
		DeleteSheet: &sheets.DeleteSheetRequest{
			SheetId: g.SheetID,
		},
	}

	buRequest = &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{&req},
	}

	_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()

	if err != nil {
		result = GenError(err.Error())
		return
	} else {
		result.Result = true
	}

	return
}

// SheetAdd добавляет вкладку
func (g *GoogleSheet) SheetAdd() (result Result) {

	var req sheets.Request
	var buRequest *sheets.BatchUpdateSpreadsheetRequest
	var err error

	req = sheets.Request{
		AddSheet: &sheets.AddSheetRequest{
			Properties: &sheets.SheetProperties{
				Index: 0,
				Title: workTab,
			},
		},
	}

	buRequest = &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{&req},
	}

	_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()

	if err != nil {
		result = GenError(err.Error())
	} else {
		result.Result = true
	}
	return
}

// ColumnDelete удаляет столбец
func (g *GoogleSheet) ColumnDelete(limit int64) (result Result) {

	var requests []*sheets.Request
	var req sheets.Request
	var buRequest *sheets.BatchUpdateSpreadsheetRequest
	var err error

	req = sheets.Request{
		InsertDimension: &sheets.InsertDimensionRequest{
			Range: &sheets.DimensionRange{
				Dimension:  "COLUMNS",
				SheetId:    g.SheetID,
				StartIndex: limit + 1,
				EndIndex:   limit + 2,
			},
			InheritFromBefore: true,
		},
	}

	buRequest = &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{&req},
	}

	_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()

	if err != nil {
		result = GenError(err.Error())
		return
	}

	req = sheets.Request{
		DeleteDimension: &sheets.DeleteDimensionRequest{
			Range: &sheets.DimensionRange{
				Dimension:  "COLUMNS",
				SheetId:    g.SheetID,
				StartIndex: 0,
				EndIndex:   limit,
			},
		},
	}
	requests = append(requests, &req)

	buRequest = &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}
	_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()

	if err != nil {
		result = GenError(err.Error())
	} else {
		result.Result = true
	}
	return
}

// GroupInsert добавляет название анкеты
func (g *GoogleSheet) GroupInsert(actions []GoogleAction) (result Result) {

	var err error
	var reqs []*sheets.Request
	var buRequest *sheets.BatchUpdateSpreadsheetRequest

	for _, action := range actions {
		reqs = append(reqs, &sheets.Request{
			UpdateCells: &sheets.UpdateCellsRequest{
				Range: &sheets.GridRange{
					StartRowIndex:    0,
					EndRowIndex:      1,
					StartColumnIndex: action.IndexStart,
					EndColumnIndex:   action.IndexStart + 1,
					SheetId:          g.SheetID,
				},
				Rows: []*sheets.RowData{
					&sheets.RowData{
						Values: []*sheets.CellData{
							&sheets.CellData{
								UserEnteredValue: &sheets.ExtendedValue{
									StringValue: action.NameGroup,
								},
							},
						},
					},
				},
				Fields: "*",
			},
		})
	}

	buRequest = &sheets.BatchUpdateSpreadsheetRequest{
		Requests: reqs,
	}

	_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()

	if err != nil {
		result = GenError(err.Error())
		return
	} else {
		result.Result = true
	}

	return
}

// ColumnInsert добавляет столбец
func (g *GoogleSheet) ColumnInsert(actions []GoogleAction) (result Result) {

	var err error
	var reqs []*sheets.Request
	var buRequest *sheets.BatchUpdateSpreadsheetRequest

	for _, action := range actions {
		reqs = append(reqs, &sheets.Request{
			InsertDimension: &sheets.InsertDimensionRequest{
				Range: &sheets.DimensionRange{
					Dimension:  "COLUMNS",
					SheetId:    g.SheetID,
					StartIndex: action.IndexStart + 1,
					EndIndex:   action.IndexStart + 2,
				},
				InheritFromBefore: true,
			},
		})
	}

	buRequest = &sheets.BatchUpdateSpreadsheetRequest{
		Requests: reqs,
	}
	_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()
	if err != nil {
		result = GenError(err.Error())
		return
	}

	for _, action := range actions {
		reqs = append(reqs, &sheets.Request{
			UpdateCells: &sheets.UpdateCellsRequest{
				Range: &sheets.GridRange{
					StartRowIndex:    1,
					EndRowIndex:      2,
					StartColumnIndex: action.IndexStart,
					EndColumnIndex:   action.IndexStart + 1,
					SheetId:          g.SheetID,
				},
				Rows: []*sheets.RowData{
					&sheets.RowData{
						Values: []*sheets.CellData{
							&sheets.CellData{
								UserEnteredValue: &sheets.ExtendedValue{
									StringValue: action.NameColumn,
								},
							},
						},
					},
				},
				Fields: "*",
			},
		})
	}
	buRequest = &sheets.BatchUpdateSpreadsheetRequest{
		Requests: reqs,
	}
	_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()
	if err != nil {
		result = GenError(err.Error())
		return
	} else {
		result.Result = true
	}

	return
}

// RowsDelete удаляет строки
func (g *GoogleSheet) RowsDelete(rows []int) (result Result) {
	var err error
	var reqs []*sheets.Request
	var buRequest *sheets.BatchUpdateSpreadsheetRequest
	for _, row := range rows {
		reqs = append(reqs, &sheets.Request{
			DeleteRange: &sheets.DeleteRangeRequest{
				ShiftDimension: "ROWS",
				Range: &sheets.GridRange{
					StartRowIndex: int64(row),
					EndRowIndex:   int64(row + 1),
					SheetId:       g.SheetID,
				},
			},
		})
	}

	if len(reqs) > 0 {

		buRequest = &sheets.BatchUpdateSpreadsheetRequest{
			Requests: reqs,
		}

		_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()

		if err != nil {
			result = GenError(err.Error())
		} else {
			result.Result = true
		}

	}

	return
}

// RowsInsert добавляет строки
func (g *GoogleSheet) RowsInsert(key int64, data []GoogleRow) (result Result) {

	var offset int64 = 2

	var err error
	var reqs []*sheets.Request
	var buRequest *sheets.BatchUpdateSpreadsheetRequest

	var row int64
	for _, item := range data {
		for column, answer := range item.Data {

			reqs = append(reqs, &sheets.Request{
				UpdateCells: &sheets.UpdateCellsRequest{
					Range: &sheets.GridRange{
						StartRowIndex:    key + row + offset,
						EndRowIndex:      key + row + offset + 1,
						StartColumnIndex: int64(column),
						EndColumnIndex:   int64(column + 1),
						SheetId:          g.SheetID,
					},
					Rows: []*sheets.RowData{
						&sheets.RowData{
							Values: []*sheets.CellData{
								&sheets.CellData{
									UserEnteredValue: &sheets.ExtendedValue{
										StringValue: answer.Value,
									},
								},
							},
						},
					},
					Fields: "*",
				},
			})

		}
		row++
	}

	if len(reqs) > 0 {

		buRequest = &sheets.BatchUpdateSpreadsheetRequest{
			Requests: reqs,
		}

		_, err = g.Service.Spreadsheets.BatchUpdate(g.Spreadsheet, buRequest).Do()

		if err != nil {
			result = GenError(err.Error())
		} else {
			result.Result = true
		}

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
