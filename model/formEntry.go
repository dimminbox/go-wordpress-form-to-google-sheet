package model

import "github.com/jinzhu/gorm"

//FormEntry - результат заполнения опросника
type FormEntry struct {
	gorm.Model
	ID            int    `gorm:"column:entries_id; primary_key"`
	FormID        int    `gorm:"column:form_id"`
	Data          string `gorm:"column:data"`
	Subject       string `gorm:"column:subject"`
	SenderName    string `gorm:"column:sender_name"`
	SenderEmail   string `gorm:"column:sender_email"`
	EmailsTo      string `gorm:"column:emails_to"`
	DateSubmitted string `gorm:"column:date_submitted"`
	IPAddress     string `gorm:"column:	ip_address"`
	Approved      string `gorm:"column:	entry_approved"`
}

//TableName получения таблицы БД
func (FormEntry) TableName() string {
	return "wp_visual_form_builder_entries"
}
