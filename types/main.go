package types

type Sentence struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Sentence string `json:"sentence"`
}

type Sentences []Sentence