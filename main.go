package main

import (
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

// GoogleAction структура для хранения действия по Google таблице
type GoogleAction struct {
	Action      string //insert, delete
	Type        string //column, group
	NameColumn  string //название колонки
	NameGroup   string //название группы
	IndexDelete int64  //номер колонки для удаления
	IndexStart  int64  //номер колонки относительно которой нужно вставить данные

}

func main() {

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
				break
			}

			var keys map[int]string
			keys, result = google.Keys()
			if !result.Result {
				fmt.Println(result.Error)
				break
			}

			var wpColumns map[string][]string
			wpColumns, result = getWPColumns()
			if !result.Result {
				fmt.Println(result.Error)
				break
			}
			//fmt.Printf("WP Columns %+v\n\n", wpColumns)

			// получаем стобцы из Google
			var googleColumns map[string][]string
			googleColumns, result = google.Columns()
			if !result.Result {
				fmt.Println(result.Error)
				break
			}
			//fmt.Printf("Google columns %+v\n\n", googleColumns)

			// приводим структуру таблицы в Google к виду из Wordpress
			// убираем лишние колонки, добавляем нужные

			googleColumns, result = google.Columns()
			if !result.Result {
				fmt.Println(result.Error)
				break
			}
			result = makeGoogleColumns(google, wpColumns, googleColumns)
			fmt.Printf("%+v", result)
			// получаем анкеты из WP
			responses := responsesNew(keys)

			result = addGoogleResults(google, keys, responses)
			fmt.Printf("%+v", result)

			checkSumOrig = Checksum
		}

		d := time.Duration(60) * time.Second
		time.Sleep(d)

	}

}

func getWPColumns() (columns map[string][]string, result model.Result) {

	columns = map[string][]string{}

	config := model.Configure()

	//googleCells := model.GetColumnNames(2)
	var entries []model.FormEntry
	model.Connect.Find(&entries)

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

	var actions []GoogleAction
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
			actions = append(actions, GoogleAction{Action: "insert", NameGroup: title, Type: "group", IndexStart: (lastGoogleColumn + offset)})
			for _, wpColumn := range response {
				actions = append(actions, GoogleAction{Action: "insert", Type: "column", NameColumn: wpColumn, IndexStart: (lastGoogleColumn + offset)})
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
					actions = append(actions, GoogleAction{Action: "insert", Type: "column", NameColumn: wpColumn, IndexStart: startColumn})
				}

			}

		}

	}

	fmt.Printf("%+v", actions)
	// разбираемся с колонками
	for _, action := range actions {
		if action.Type == "column" {
			switch action.Action {

			case "insert":
				result = google.ColumnInsert(action.NameColumn, action.IndexStart)
				if !result.Result {
					fmt.Printf("%+v\n", result)
					return
				}
			}

		} else if action.Type == "group" {

			if action.Action == "insert" {
				result = google.GroupInsert(action.NameGroup, action.IndexStart)
				if !result.Result {
					fmt.Printf("%+v\n", result)
					return
				}
			}

		}
	}

	result.Result = true
	return
}

func responsesNew(keys map[int]string) (result map[string][]model.AnswerOption) {

	result = map[string][]model.AnswerOption{}

	config := model.Configure()
	//var answerOptionNew []model.AnswerOption

	var entries []model.FormEntry
	model.Connect.Find(&entries)

	var uniqKey string
	for _, entry := range entries {

		var form model.Form
		model.Connect.
			Where("form_id = ?", entry.FormID).
			Find(&form)
		if form.FormID == 0 {
			fmt.Printf("Для опросника %d не найдена форма.\n", entry.ID)
			return
		}

		//str = `a:16:{i:0;a:7:{s:2:"id";s:2:"10";s:4:"slug";s:2:"10";s:4:"name";s:0:"";s:4:"type";s:8:"fieldset";s:7:"options";s:0:"";s:9:"parent_id";s:1:"0";s:5:"value";s:0:"";}i:1;a:7:{s:2:"id";s:1:"7";s:4:"slug";s:61:"d0bed0b1d189d0b0d18f-d0b8d0bdd184d0bed180d0bcd0b0d186d0b8d18f";s:4:"name";s:31:"Общая информация";s:4:"type";s:7:"section";s:7:"options";s:0:"";s:9:"parent_id";s:2:"10";s:5:"value";s:0:"";}i:2;a:7:{s:2:"id";s:2:"12";s:4:"slug";s:57:"d0b4d0b0d182d0b0-d0b7d0b0d0bfd0bed0bbd0bdd0b5d0bdd0b8d18f";s:4:"name";s:29:"Дата заполнения";s:4:"type";s:4:"date";s:7:"options";s:39:"a:1:{s:10:"dateFormat";s:8:"mm/dd/yy";}";s:9:"parent_id";s:1:"7";s:5:"value";s:10:"07/05/2019";}i:3;a:7:{s:2:"id";s:2:"14";s:4:"slug";s:49:"d0b4d0b0d182d0b0-d180d0bed0b6d0b4d0b5d0bdd0b8d18f";s:4:"name";s:25:"Дата рождения";s:4:"type";s:4:"date";s:7:"options";s:39:"a:1:{s:10:"dateFormat";s:8:"mm/dd/yy";}";s:9:"parent_id";s:1:"7";s:5:"value";s:10:"07/04/2019";}i:4;a:7:{s:2:"id";s:2:"15";s:4:"slug";s:74:"d184d0b0d0bcd0b8d0bbd0b8d18f-d0b8d0bcd18f-d0bed182d187d0b5d181d182d0b2d0be";s:4:"name";s:40:"Фамилия, имя, отчество";s:4:"type";s:4:"text";s:7:"options";s:0:"";s:9:"parent_id";s:1:"7";s:5:"value";s:5:"13123";}i:5;a:7:{s:2:"id";s:2:"18";s:4:"slug";s:40:"d0b8d0bdd181d182d180d183d0bad186d0b8d18f";s:4:"name";s:20:"Инструкция";s:4:"type";s:12:"instructions";s:7:"options";s:0:"";s:9:"parent_id";s:2:"10";s:5:"value";s:0:"";}i:6;a:7:{s:2:"id";s:2:"20";s:4:"slug";s:32:"d181d0b8d0bcd0bfd182d0bed0bcd18b";s:4:"name";s:16:"Симптомы";s:4:"type";s:12:"instructions";s:7:"options";s:0:"";s:9:"parent_id";s:2:"10";s:5:"value";s:0:"";}i:7;a:7:{s:2:"id";s:2:"24";s:4:"slug";s:28:"d0b2d0bed0bfd180d0bed181d18b";s:4:"name";s:14:"Вопросы";s:4:"type";s:7:"section";s:7:"options";s:0:"";s:9:"parent_id";s:2:"10";s:5:"value";s:0:"";}i:8;a:7:{s:2:"id";s:2:"23";s:4:"slug";s:75:"d0bed182d191d187d0bdd0be-d0bbd0b8-d0b2d0b0d188d0b5-d0bad0bed0bbd0b5d0bdd0be";s:4:"name";s:40:"Отёчно ли Ваше колено?";s:4:"type";s:5:"radio";s:7:"options";s:128:"a:5:{i:0;s:14:"Никогда";i:1;s:14:"Изредка";i:2;s:12:"Иногда";i:3;s:10:"Часто";i:4;s:12:"Всегда";}";s:9:"parent_id";s:2:"24";s:5:"value";s:14:"Никогда";}i:9;a:7:{s:2:"id";s:2:"25";s:4:"slug";s:134:"d0bed189d183d189d0b0d0b5d182d0b5-d0bbd0b8-d0b2d18b-d185d180d183d181d182-d181d0bbd18bd188d0b8d182d0b5-d0bbd0b8-d189d0b5d0bbd187d0bad0b8";s:4:"name";s:164:"Ощущаете ли Вы хруст, слышите ли щелчки или другие звуки при движениях в коленном суставе?";s:4:"type";s:5:"radio";s:7:"options";s:128:"a:5:{i:0;s:14:"Никогда";i:1;s:14:"Изредка";i:2;s:12:"Иногда";i:3;s:10:"Часто";i:4;s:12:"Всегда";}";s:9:"parent_id";s:2:"24";s:5:"value";s:14:"Никогда";}i:10;a:7:{s:2:"id";s:2:"26";s:4:"slug";s:134:"d0b1d18bd0b2d0b0d18ed182-d0bbd0b8-d183-d0b2d0b0d181-d0b1d0bbd0bed0bad0b0d0b4d18b-d0bad0bed0bbd0b5d0bdd0bdd0bed0b3d0be-d181d183d181d182";s:4:"name";s:144:"Бывают ли у Вас блокады коленного сустава в положении сгибания или разгибаний?";s:4:"type";s:5:"radio";s:7:"options";s:128:"a:5:{i:0;s:14:"Никогда";i:1;s:14:"Изредка";i:2;s:12:"Иногда";i:3;s:10:"Часто";i:4;s:12:"Всегда";}";s:9:"parent_id";s:2:"24";s:5:"value";s:14:"Никогда";}i:11;a:7:{s:2:"id";s:2:"27";s:4:"slug";s:132:"d0bfd0bed0bbd0bdd0bed181d182d18cd18e-d0bbd0b8-d0b2d18b-d0b2d18bd0bfd180d18fd0bcd0bbd18fd0b5d182d0b5-d180d0b0d0b7d0b3d0b8d0b1d0b0d0b5";s:4:"name";s:88:"Полностью ли Вы выпрямляете (разгибаете) колено?";s:4:"type";s:5:"radio";s:7:"options";s:128:"a:5:{i:0;s:12:"Всегда";i:1;s:10:"Часто";i:2;s:12:"Иногда";i:3;s:14:"Изредка";i:4;s:14:"Никогда";}";s:9:"parent_id";s:2:"24";s:5:"value";s:12:"Всегда";}i:12;a:7:{s:2:"id";s:2:"28";s:4:"slug";s:112:"d0bfd0bed0bbd0bdd0bed181d182d18cd18e-d0bbd0b8-d0b2d18b-d181d0b3d0b8d0b1d0b0d0b5d182d0b5-d0bad0bed0bbd0b5d0bdd0be";s:4:"name";s:59:"Полностью ли Вы сгибаете колено?";s:4:"type";s:5:"radio";s:7:"options";s:128:"a:5:{i:0;s:12:"Всегда";i:1;s:10:"Часто";i:2;s:12:"Иногда";i:3;s:14:"Изредка";i:4;s:14:"Никогда";}";s:9:"parent_id";s:2:"24";s:5:"value";s:12:"Всегда";}i:13;a:7:{s:2:"id";s:1:"2";s:4:"slug";s:12:"verification";s:4:"name";s:12:"Verification";s:4:"type";s:12:"verification";s:7:"options";s:0:"";s:9:"parent_id";s:1:"0";s:5:"value";s:0:"";}i:14;a:7:{s:2:"id";s:1:"3";s:4:"slug";s:27:"please-enter-any-two-digits";s:4:"name";s:27:"Please enter any two digits";s:4:"type";s:6:"secret";s:7:"options";s:0:"";s:9:"parent_id";s:1:"2";s:5:"value";s:2:"12";}i:15;a:7:{s:2:"id";s:1:"4";s:4:"slug";s:6:"submit";s:4:"name";s:6:"Submit";s:4:"type";s:6:"submit";s:7:"options";s:0:"";s:9:"parent_id";s:1:"2";s:5:"value";s:0:"";}}`
		out, err := exec.Command(config.PhpPath, "-r", "print json_encode(unserialize(\x22"+strings.Replace(entry.Data, "\x22", "\\x22", -1)+"\x22));").Output()
		if err == nil {

			var jsonOut string
			jsonOut = string(out)

			if jsonOut != "" {

				var answerOptions []model.AnswerOption
				json.Unmarshal([]byte(jsonOut), &answerOptions)

				isNewAnswer := true
				for _, answerOption := range answerOptions {
					for _, key := range keys {
						if key == answerOption.Value {
							isNewAnswer = false
						}
					}
				}

				if !isNewAnswer {
					continue
				}
				var answerOptionTotal []model.AnswerOption
				for _, answerOption := range answerOptions {

					if answerOption.Name == KeyName {
						uniqKey = answerOption.Value
					}

					//fmt.Printf("%+v", answerOption)
					if answerOption.Usefull() {
						answerOptionTotal = append(answerOptionTotal, answerOption)
					}

				}
				if len(result[uniqKey]) == 0 {
					result[uniqKey] = []model.AnswerOption{}
				}
				result[uniqKey] = append(result[uniqKey], answerOptionTotal...)

			}
		} else {
			fmt.Printf("%+v", err)
		}
	}

	return
}

func addGoogleResults(google model.GoogleSheet, keys map[int]string, responses map[string][]model.AnswerOption) (result model.Result) {

	for _, key := range keys {
		for uniqKey := range responses {
			if key == uniqKey {
				delete(responses, uniqKey)
			}
		}
	}
	fmt.Printf("%+v", responses)
	result = google.RowsInsert(keys, responses)

	return
}
