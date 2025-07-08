package iotp

type HistoryV2QueryData struct {
	Timestamp float64                `json:"Timestamp"`
	Fields    map[string]interface{} `json:"Fields"`
}

type DeviceMsg struct {
	Name     string   `json:"name"`
	Children []TagMsg `json:"children"`
}

type TagMsg struct {
	Name        string `json:"name"`
	Id          string `json:"id"`
	Description string `json:"description"`
}

type Operate struct {
	Type string   `json:"type"`
	Id   string   `json:"id"`
	Ids  []string `json:"ids"`
	Val  string   `json:"val"`
}

type GoRecord struct {
	Status    string  `json:"status"`
	Val       string  `json:"val"`
	Timestamp float64 `json:"timestamp"`
}

type Record struct {
	Id        string      `json:"id"`
	Status    string      `json:"status"`
	Val       interface{} `json:"val"`
	Timestamp int64       `json:"timestamp"`
}
