package database

type statement struct {
	Query      string
	Parameters map[string]any
}

type Query struct {
	Select  []string
	From    string
	Joins   []string
	Where   OperatorOfLogic
	GroupBy string
	OrderBy string
	Limit   struct {
		Count  int
		Offset int
	}
}
