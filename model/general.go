package model

import (
	"encoding/json"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

// DB - экземпляр БД
var DB *gorm.DB

// Connect - соединение с БД
var Connect *gorm.DB

//Configuration - структура конфигурации микросервиса
type Configuration struct {
	DbURI      string
	IsDebug    bool
	Spredsheet string
	PhpPath    string
}

var ExcTypes = []string{"verification", "secret", "submit", "section", "fieldset", "instructions"}

var configuration Configuration

// Configure - возвращает конфигурацию
func Configure() Configuration {
	return configuration
}

func init() {

	path := os.Getenv("GOPATH") + "/src/go-wordpress-form-to-google-sheet/config.json"
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&configuration)
	if err != nil {
		panic(err)
	}

}

//InitDB соединяется с БД и создаёт одно соединение
func InitDB() {

	// открываем новый коннект до БД если старый протух
	if Connect == nil || Connect.DB().Ping() != nil {
		fmt.Println("Open new connection to DB")
		db, err := gorm.Open("mysql", configuration.DbURI)
		db.LogMode(configuration.IsDebug)
		db.DB().SetMaxOpenConns(1000)
		db.DB().SetMaxIdleConns(0)

		if db == nil {
			panic(fmt.Sprintf("Got error when connect database, the error is '%v'", err))
		} else {
			Connect = db
		}
	}

}
