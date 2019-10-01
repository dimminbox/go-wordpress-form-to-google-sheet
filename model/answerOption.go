package model

// AnswerOption структура с ответами на форму
type AnswerOption struct {
	ID       int    `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	TypeName string `json:"type"`
	Options  string `json:"options"`
	ParentID string `json:"parent_id"`
	Value    string `json:"value"`
}


// Usefull - полезнаый ли ответ
func (a *AnswerOption) Usefull() (result bool) {
	for _, typeName := range ExcTypes {

		if a.TypeName == typeName {

			return false
		}
	}
	return true
}
