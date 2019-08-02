package model

import "github.com/jinzhu/gorm"

//Form - настройки форм
type Form struct {
	gorm.Model
	FormID int    `gorm:"column:form_id"`
	Key    string `gorm:"column:form_key"`
	Title  string `gorm:"column:form_title"`
}

//TableName получения таблицы БД
func (Form) TableName() string {
	return "wp_visual_form_builder_forms"
}
