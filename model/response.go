package model

//Result - структура для хранения результата
type Result struct {
	Error  string
	Result bool
}

// GenError какой - то очень простой генератор объекта ошибок
func GenError(err string) (result Result) {
	result.Result = false
	result.Error = err
	return
}
