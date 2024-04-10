package spider

type RuleTree struct {
	Root  func() ([]*Request, error)
	Trunk map[string]*Rule
}

type Rule struct {
	ItemFields []string
	ParseFunc  func(*Context) (ParseResult, error)
}
