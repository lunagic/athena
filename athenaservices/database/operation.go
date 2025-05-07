package database

type OperatorOfLogic interface {
	OperatorOfEvaluation
	hasAny() bool
}

type simpleOperatorOfLogic struct {
	operatorKeyword     string
	operatorsEvaluation []OperatorOfEvaluation
}

func (o simpleOperatorOfLogic) hasAny() bool {
	return len(o.operatorsEvaluation) > 0
}

func (o simpleOperatorOfLogic) haveDriverRender(driver Driver) (statement, error) {
	return driver.generateSimpleOperatorOfLogic(o)
}

type OperatorOfEvaluation interface {
	haveDriverRender(driver Driver) (statement, error)
}

func And(operatorsEvaluation ...OperatorOfEvaluation) OperatorOfLogic {
	return simpleOperatorOfLogic{
		operatorKeyword:     "AND",
		operatorsEvaluation: operatorsEvaluation,
	}
}

func Or(operatorsEvaluation ...OperatorOfEvaluation) OperatorOfLogic {
	return simpleOperatorOfLogic{
		operatorKeyword:     "OR",
		operatorsEvaluation: operatorsEvaluation,
	}
}

func Equal[T any](column *T, value T) OperatorOfEvaluation {
	return simpleOperatorOfEquality{
		Column:   column,
		Operator: "=",
		Value:    value,
	}
}
func GreaterThan[T any](column *T, value T) OperatorOfEvaluation {
	return simpleOperatorOfEquality{
		Column:   column,
		Operator: ">",
		Value:    value,
	}
}

func GreaterThanOrEqual[T any](column *T, value T) OperatorOfEvaluation {
	return simpleOperatorOfEquality{
		Column:   column,
		Operator: ">=",
		Value:    value,
	}
}

func LessThan[T any](column *T, value T) OperatorOfEvaluation {
	return simpleOperatorOfEquality{
		Column:   column,
		Operator: "<",
		Value:    value,
	}
}

func LessThanOrEqual[T any](column *T, value T) OperatorOfEvaluation {
	return simpleOperatorOfEquality{
		Column:   column,
		Operator: "<=",
		Value:    value,
	}
}

func NotEqual[T any](column *T, value T) OperatorOfEvaluation {
	return simpleOperatorOfEquality{
		Column:   column,
		Operator: "!=",
		Value:    value,
	}
}

func Like[T any](column *T, value T) OperatorOfEvaluation {
	return simpleOperatorOfEquality{
		Column:   column,
		Operator: "LIKE",
		Value:    value,
	}
}

type simpleOperatorOfEquality struct {
	Column   any
	Operator string
	Value    any
}

func (o simpleOperatorOfEquality) haveDriverRender(driver Driver) (statement, error) {
	return driver.generateSimpleOperatorOfEquality(o)
}
