package nullplatform

type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Paging struct {
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}
