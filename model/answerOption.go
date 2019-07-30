package model

// AnswerOption структура с ответами на форму
type AnswerOption struct {
	ID       int `json:"id"`
	Slug     string
	Name     string
	TypeName string
	Options  string
	ParentID string `json:"parent_id"`
	Value    string
}
