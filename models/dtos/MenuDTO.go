package dtos

type MenuDTO struct {
	Id             int64                  `json:"id"`
	Name           string                 `json:"name"`
	Path           string                 `json:"path"`
	Component      string                 `json:"component"`
	ParentId       *int64                 `json:"parentId"`
	Status         int                    `json:"status"`
	Type           int                    `json:"type"`
	Meta           map[string]interface{} `json:"meta"`
	PermissionList map[string]interface{} `json:"permissionList"`
	Redirect       string                 `json:"redirect"`
	Priority       int                    `json:"priority"`
	Children       []MenuDTO              `json:"children,omitempty"`
}
