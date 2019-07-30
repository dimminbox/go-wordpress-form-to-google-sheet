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

	data, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		result = GenError(err.Error())
		return
	}

	/*conf, err := google.JWTConfigFromJSON(data, spreadsheet.Scope)
	if err != nil {
		result = GenError(err.Error())
		return
	}*/

	config, err := google.ConfigFromJSON(data, "https://www.googleapis.com/auth/spreadsheets")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := g.getClient(config)

	srv, err := sheets.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	// Prints the names and majors of students in a sample spreadsheet:
	// https://docs.google.com/spreadsheets/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/edi
	readRange := "Response!A2:E"
	resp, err := srv.Spreadsheets.Values.Get(g.Spreadsheet, readRange).Do()
	if err != nil {
		result = GenError(err.Error())
		return
	} else {
		result.Result = true
	}

	fmt.Printf("%+v", resp)
	/*client := conf.Client(context.TODO())
	service := spreadsheet.NewServiceWithClient(client)

	spreadsheet, err := service.FetchSpreadsheet(g.Spreadsheet)
	if err != nil {
		result = GenError(err.Error())
		return
	}

	sheet, err := spreadsheet.SheetByIndex(0)
	if err == nil {
		g.Sheet = *sheet
		result.Result = true
	} else {
		result = GenError(err.Error())
	}*/

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
