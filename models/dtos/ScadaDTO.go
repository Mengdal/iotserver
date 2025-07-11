package dtos

type AlarmRecordReq struct {
	Page      int    `json:"page"`
	Size      int    `json:"size"`
	Status    string `json:"status"`
	StartTime string `json:"starttime"`
	EndTime   string `json:"endtime"`
}
