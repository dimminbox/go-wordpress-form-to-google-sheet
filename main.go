package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	model "go-wordpress-form-to-google-sheet/model"
	"os/exec"
	"strings"
	"time"
)

// KeyName - ключевое поле в опроснике по которому все они сводятся
const KeyName = "Номер моб. телефона"

// TableHead тип для заголовка таблицы
type TableHead map[string][]string

func main() {

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()

	model.InitDB()
	config := model.Configure()

	var result model.Result
	var table string
	var checkSumOrig string

	for true {

		var Checksum string
		row := model.Connect.Raw("CHECKSUM TABLE wp_visual_form_builder_entries").Row()
		row.Scan(&table, &Checksum)

		if Checksum != checkSumOrig {

			google := model.GoogleSheet{
				Spreadsheet: config.Spredsheet,
			}

			result = google.Init()
			if !result.Result {
				fmt.Println(result.Error)
				wait()
				continue
			}

			var wpColumns map[string][]string
			wpColumns, result = getWPColumns()
			if !result.Result {
				fmt.Println(result.Error)
				wait()
				continue
			}
			//fmt.Printf("WP Columns %+v\n\n", wpColumns)

			// получаем стобцы из Google
			var googleColumns map[string][]string
			googleColumns, result = google.Columns()
			if !result.Result {
				fmt.Println(result.Error)
				wait()
				continue
			}

			var gRows []model.GoogleRow
			gRows, result = google.Data()
			if !result.Result {
				fmt.Println(result.Error)
				wait()
				continue
			}

			// получаем анкеты из WP
			responses := responses(googleColumns)

			// приводим структуру таблицы в Google к виду из Wordpress
			// убираем лишние колонки, добавляем нужные

			result = makeGoogleColumns(google, wpColumns, googleColumns)
			if !result.Result {
				result = makeGoogleResults(google, gRows, responses)
			}

			checkSumOrig = Checksum
		}

		wait()
	}
}

func wait() {
	const num = 60
	fmt.Printf("waiting %d seconds.\n", num)
	d := time.Duration(num) * time.Second
	time.Sleep(d)
}
func getWPColumns() (columns map[string][]string, result model.Result) {

	fmt.Println("getWPColumns")
	columns = map[string][]string{}

	config := model.Configure()

	//googleCells := model.GetColumnNames(2)
	var entries []model.FormEntry
	model.Connect.
		Where("entry_approved = ?", 1).
		Order("form_id DESC").
		Find(&entries)

	for _, entry := range entries {

		var form model.Form
		model.Connect.
			Where("form_id = ?", entry.FormID).
			Find(&form)
		if form.FormID == 0 {
			result = model.GenError(fmt.Sprintf("Для опросника %d не найдена форма.\n", entry.ID))
			return
		}

		columns[form.Title] = []string{}

		out, err := exec.Command(config.PhpPath, "-r", "print json_encode(unserialize(\x22"+strings.Replace(entry.Data, "\x22", "\\x22", -1)+"\x22));").Output()
		if err != nil {
			result = model.GenError(err.Error())
			return
		}

		var jsonOut string
		jsonOut = string(out)

		if jsonOut == "" {
			continue
		}
		var answerOptions []model.AnswerOption
		json.Unmarshal([]byte(jsonOut), &answerOptions)

		for _, answerOption := range answerOptions {
			if answerOption.Usefull() {
				columns[form.Title] = append(columns[form.Title], answerOption.Name)
			}

		}

	}

	result.Result = true

	return

}
func makeCellNames() (result []string) {

	for i := 0; i < 26; i++ {
		result = append(result, string('A'-1+i)+"1")
	}

	for i := 0; i < 26; i++ {
		for j := 0; j < 26; j++ {
			result = append(result, fmt.Sprintf("%s%s1", string('A'-1+i), string('A'-1+j)))
		}
	}

	return
}

func columnsWasDifferents(wpColumns TableHead, googleColumns TableHead) (result bool) {

	for title, response := range wpColumns {

		if googleAnswers, ok := googleColumns[title]; !ok {
			result = true
			return
		} else {

			for _, wpColumn := range response {

				// проверям на то что колонка новая в анкете
				var isset bool
				for _, googleAnswer := range googleAnswers {
					if wpColumn == googleAnswer {
						isset = true
						break
					}
				}
				if !isset {
					result = true
					return
				}

			}
		}
	}

	return

}
func makeGoogleColumns(google model.GoogleSheet, wpColumns TableHead, googleColumns TableHead) (result model.Result) {

	fmt.Println("makeGoogleColumns")
	var actionGroup []model.GoogleAction
	var actionColumn []model.GoogleAction

	if columnsWasDifferents(wpColumns, googleColumns) {

		// удаляем вкладку
		result = google.SheetRemove()
		if !result.Result {
			return
		}

		//  Добавляем вкладку и устнавливаем её рабочей
		result = google.Init()
		if !result.Result {
			return
		}
	}

	// вычисление последнего столбца для добавления новых анкет в самый конец если таково будет нужно
	var lastGoogleColumn int64
	for title := range googleColumns {
		lastGoogleColumn += int64(len(googleColumns[title]))
	}

	var offset int64

	// проходимся по колонкам из WP для добавления колонок в Google
	for title, response := range wpColumns {

		if googleAnswers, ok := googleColumns[title]; !ok {

			//в Google не существует анкеты
			actionGroup = append(actionGroup, model.GoogleAction{Action: "insert", NameGroup: title, Type: "group", IndexStart: (lastGoogleColumn + offset)})
			for _, wpColumn := range response {
				actionColumn = append(actionColumn, model.GoogleAction{Action: "insert", Type: "column", NameColumn: wpColumn, IndexStart: (lastGoogleColumn + offset)})
				offset++
			}

		} else {

			for _, wpColumn := range response {

				// проверям на то что колонка новая в анкете
				var isset bool
				for _, googleAnswer := range googleAnswers {
					if wpColumn == googleAnswer {
						isset = true
						break
					}
				}

				var startColumn int64
				for googleColumn := range googleColumns {

					if title == googleColumn {
						startColumn += int64(len(googleColumns[googleColumn]))
						break
					} else {
						startColumn += int64(len(googleColumns[googleColumn]))
					}
				}

				// добавляем новую колонку в текущей анкете
				if !isset {
					actionColumn = append(actionColumn, model.GoogleAction{Action: "insert", Type: "column", NameColumn: wpColumn, IndexStart: startColumn})
				}

			}

		}

	}
	if len(actionColumn) > 0 {
		result = google.ColumnInsert(actionColumn)
		if !result.Result {
			return
		}
	}

	if len(actionGroup) > 0 {
		result = google.GroupInsert(actionGroup)
		if !result.Result {
			return
		}
	}

	if len(actionColumn) > 0 || len(actionGroup) > 0 {
		result.Result = true
	} else {
		result.Result = false
	}
	return
}

func responses(keys map[string][]string) (result map[string]model.GoogleRow) {

	result = map[string]model.GoogleRow{}

	config := model.Configure()

	for nameForm, gAnswers := range keys {
		var form model.Form
		model.Connect.
			Where("form_title = ?", nameForm).
			Find(&form)

		var uniqKey string

		var entries []model.FormEntry
		model.Connect.
			Where("entry_approved = ?", 1).
			Where("form_id = ?", form.FormID).
			Find(&entries)

		for _, entry := range entries {

			out, err := exec.Command(config.PhpPath, "-r", "print json_encode(unserialize(\x22"+strings.Replace(entry.Data, "\x22", "\\x22", -1)+"\x22));").Output()
			if err == nil {

				var jsonOut string
				jsonOut = string(out)

				if jsonOut != "" {

					var answerOptions []model.AnswerOption
					json.Unmarshal([]byte(jsonOut), &answerOptions)

					var answerOptionTotal []model.AnswerOption
					for _, key := range gAnswers {

						value := model.AnswerOption{
							Name: key,
						}
						for _, answerOption := range answerOptions {

							if answerOption.Name == KeyName {
								uniqKey = answerOption.Value
							}
							if answerOption.Name == key {
								value = answerOption
								break
							}
						}
						answerOptionTotal = append(answerOptionTotal, value)
					}

					if _, ok := result[uniqKey]; !ok {
						result[uniqKey] = model.GoogleRow{
							Index: uniqKey,
							Data:  []model.AnswerOption{},
						}
					}

					temp := result[uniqKey]
					temp.Data = append(result[uniqKey].Data, answerOptionTotal...)
					result[uniqKey] = temp

				}
			} else {
				fmt.Printf("%+v", err)
			}

		}

	}

	for index, item := range result {
		var items string

		for _, answer := range item.Data {
			items = fmt.Sprintf("%s%s", items, answer.Value)
		}

		hasher := md5.New()
		hasher.Write([]byte(items))

		temp := item
		temp.Checksum = hex.EncodeToString(hasher.Sum(nil))
		result[index] = temp

	}
	return
}

func makeGoogleResults(google model.GoogleSheet, gResponses []model.GoogleRow, wpResponses map[string]model.GoogleRow) (result model.Result) {

	fmt.Println("makeGoogleResults")

	var rowsToRemove []int
	var newReponses []model.GoogleRow
	for _, wpResponse := range wpResponses {

		newAnswer := true
		for index, gResponse := range gResponses {

			if gResponse.Index == "" {
				rowsToRemove = append(rowsToRemove, index+2)
			} else if gResponse.Index == wpResponse.Index {
				if gResponse.Checksum == wpResponse.Checksum {
					newAnswer = false
					break
				} else {
					// удаляем строку из гугла
					rowsToRemove = append(rowsToRemove, index+2)
					newAnswer = true
				}
			}
		}
		if newAnswer {
			newReponses = append(newReponses, wpResponse)
		}

	}

	if len(rowsToRemove) > 0 {
		fmt.Printf("rowsToRemove %+v\n", rowsToRemove)
		result = google.RowsDelete(rowsToRemove)
		fmt.Printf("result %+v\n", result)
		return
	}

	if len(newReponses) > 0 {
		fmt.Printf("newReponses %+v\n", newReponses)
		key := int64(len(gResponses))
		result = google.RowsInsert(key, newReponses)
		fmt.Printf("result %+v\n", result)
		return
	}
	return
}
