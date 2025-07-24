package routers

import (
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context/param"
)

func init() {

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:HistoryController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:HistoryController"],
		beego.ControllerComments{
			Method:           "AggQueryHistory",
			Router:           `/aggQueryHistory`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:HistoryController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:HistoryController"],
		beego.ControllerComments{
			Method:           "BoolQueryHistory",
			Router:           `/boolQueryHistory`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:HistoryController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:HistoryController"],
		beego.ControllerComments{
			Method:           "DiffQueryHistory",
			Router:           `/diffQueryHistory`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:HistoryController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:HistoryController"],
		beego.ControllerComments{
			Method:           "QueryHistory",
			Router:           `/queryHistory`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"],
		beego.ControllerComments{
			Method:           "AddTag",
			Router:           `/addTag`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"],
		beego.ControllerComments{
			Method:           "GetTagValue",
			Router:           `/getTagValue`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"],
		beego.ControllerComments{
			Method:           "ListDevicesByTag",
			Router:           `/listDevicesByTag`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"],
		beego.ControllerComments{
			Method:           "ListTagsByDevice",
			Router:           `/listTagsByDevice`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:LabelController"],
		beego.ControllerComments{
			Method:           "RemoveTag",
			Router:           `/removeTag`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:RealController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:RealController"],
		beego.ControllerComments{
			Method:           "QueryReal",
			Router:           `/queryReal`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:TagController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:TagController"],
		beego.ControllerComments{
			Method:           "GetTagsTree",
			Router:           `/getTagsTree`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers/edgeController:WebsocketController"] = append(beego.GlobalControllerRouter["iotServer/controllers/edgeController:WebsocketController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/`,
			AllowHTTPMethods: []string{"get"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:AlertController"] = append(beego.GlobalControllerRouter["iotServer/controllers:AlertController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/delete`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:AlertController"] = append(beego.GlobalControllerRouter["iotServer/controllers:AlertController"],
		beego.ControllerComments{
			Method:           "Detail",
			Router:           `/detail`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:AlertController"] = append(beego.GlobalControllerRouter["iotServer/controllers:AlertController"],
		beego.ControllerComments{
			Method:           "GetAlarmRecord",
			Router:           `/getAlarmRecord`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:AlertController"] = append(beego.GlobalControllerRouter["iotServer/controllers:AlertController"],
		beego.ControllerComments{
			Method:           "UpdateStatus",
			Router:           `/updateStatus`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DeviceController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DeviceController"],
		beego.ControllerComments{
			Method:           "GetAllDevices",
			Router:           `/all`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DeviceController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DeviceController"],
		beego.ControllerComments{
			Method:           "Bind",
			Router:           `/bind`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DeviceController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DeviceController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/delete`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DeviceController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DeviceController"],
		beego.ControllerComments{
			Method:           "GetDevice",
			Router:           `/get`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DeviceController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DeviceController"],
		beego.ControllerComments{
			Method:           "Update",
			Router:           `/update`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"],
		beego.ControllerComments{
			Method:           "AlertCallback",
			Router:           `/callback`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"],
		beego.ControllerComments{
			Method:           "CreateRule",
			Router:           `/create`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"],
		beego.ControllerComments{
			Method:           "GetRule",
			Router:           `/rule`,
			AllowHTTPMethods: []string{"get"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"],
		beego.ControllerComments{
			Method:           "UpdateRule",
			Router:           `/update`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:MenuController"] = append(beego.GlobalControllerRouter["iotServer/controllers:MenuController"],
		beego.ControllerComments{
			Method:           "Create",
			Router:           `/create`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:MenuController"] = append(beego.GlobalControllerRouter["iotServer/controllers:MenuController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/del`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:MenuController"] = append(beego.GlobalControllerRouter["iotServer/controllers:MenuController"],
		beego.ControllerComments{
			Method:           "Edit",
			Router:           `/edit`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:MenuController"] = append(beego.GlobalControllerRouter["iotServer/controllers:MenuController"],
		beego.ControllerComments{
			Method:           "List",
			Router:           `/list`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ModelController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ModelController"],
		beego.ControllerComments{
			Method:           "Create",
			Router:           `/create`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ModelController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ModelController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/delete`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ModelController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ModelController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/get`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ProductController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ProductController"],
		beego.ControllerComments{
			Method:           "Create",
			Router:           `/create`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ProductController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ProductController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/delete`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ProductController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ProductController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/get`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ProductController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ProductController"],
		beego.ControllerComments{
			Method:           "Update",
			Router:           `/update`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RoleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RoleController"],
		beego.ControllerComments{
			Method:           "Create",
			Router:           `/create`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RoleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RoleController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/del`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RoleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RoleController"],
		beego.ControllerComments{
			Method:           "Edit",
			Router:           `/edit`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RoleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RoleController"],
		beego.ControllerComments{
			Method:           "GetRole",
			Router:           `/getRole`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RoleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RoleController"],
		beego.ControllerComments{
			Method:           "GetRoleList",
			Router:           `/getRoleList`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RoleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RoleController"],
		beego.ControllerComments{
			Method:           "Template",
			Router:           `/template`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RuleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RuleController"],
		beego.ControllerComments{
			Method:           "Edit",
			Router:           `/edit`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RuleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RuleController"],
		beego.ControllerComments{
			Method:           "OperateRule",
			Router:           `/operate`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RuleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RuleController"],
		beego.ControllerComments{
			Method:           "GetRuleStatus",
			Router:           `/status`,
			AllowHTTPMethods: []string{"get"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:RuleController"] = append(beego.GlobalControllerRouter["iotServer/controllers:RuleController"],
		beego.ControllerComments{
			Method:           "Update",
			Router:           `/update`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UserController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UserController"],
		beego.ControllerComments{
			Method:           "GetAll",
			Router:           `/get`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UserController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UserController"],
		beego.ControllerComments{
			Method:           "Login",
			Router:           `/login`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UserController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UserController"],
		beego.ControllerComments{
			Method:           "Register",
			Router:           `/register`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UserController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UserController"],
		beego.ControllerComments{
			Method:           "Update",
			Router:           `/updatePassword`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UserController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UserController"],
		beego.ControllerComments{
			Method:           "Put",
			Router:           `/updateUser`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:WriteController"] = append(beego.GlobalControllerRouter["iotServer/controllers:WriteController"],
		beego.ControllerComments{
			Method:           "Command",
			Router:           `/command`,
			AllowHTTPMethods: []string{"post"},
			MethodParams: param.Make(
				param.New("deviceCode"),
				param.New("tagCode"),
				param.New("val", param.IsRequired),
			),
			Filters: nil,
			Params:  nil})

}
