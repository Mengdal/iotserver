package models

type HistoryObject struct {
	IDs       []string `json:"ids"`
	StartTime string   `json:"startTime"`
	EndTime   string   `json:"endTime"`
	Count     int64    `json:"count"`
}

type AggObject struct {
	IDs       []string `json:"ids"`
	StartTime string   `json:"startTime"`
	EndTime   string   `json:"endTime"`
	Count     int64    `json:"count"`
	Period    int64    `json:"period"`
	AggType   string   `json:"aggType"`
	Desc      bool     `json:"desc"`
}

type DiffObject struct {
	IDs       []string `json:"ids"`
	StartTime string   `json:"startTime"`
	EndTime   string   `json:"endTime"`
	Period    int64    `json:"period"`
}

type BoolObject struct {
	IDs       []string `json:"ids"`
	StartTime string   `json:"startTime"`
	EndTime   string   `json:"endTime"`
}
