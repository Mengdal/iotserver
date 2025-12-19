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

	beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"],
		beego.ControllerComments{
			Method:           "CreateDepartment",
			Router:           `/create`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"],
		beego.ControllerComments{
			Method:           "DeleteDepartment",
			Router:           `/delete`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"],
		beego.ControllerComments{
			Method:           "Detail",
			Router:           `/detail`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"],
		beego.ControllerComments{
			Method:           "NoDepartmentDevices",
			Router:           `/noDepartmentDevices`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"],
		beego.ControllerComments{
			Method:           "GetDepartmentTree",
			Router:           `/tree`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DepartmentController"],
		beego.ControllerComments{
			Method:           "UpdateDepartment",
			Router:           `/update`,
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
			Method:           "GetNoBindDevices",
			Router:           `/devices`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DeviceController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DeviceController"],
		beego.ControllerComments{
			Method:           "GetDevicesTree",
			Router:           `/getDevicesTree`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:DeviceController"] = append(beego.GlobalControllerRouter["iotServer/controllers:DeviceController"],
		beego.ControllerComments{
			Method:           "GetTagsTree",
			Router:           `/getTagsTree`,
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
			Method:           "SceneCallback",
			Router:           `/callback2`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EkuiperController"],
		beego.ControllerComments{
			Method:           "EngineCallback",
			Router:           `/callback3`,
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

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "ControllerEngine",
			Router:           `/controllerEngine`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "DelEngine",
			Router:           `/delEngine`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "DelSource",
			Router:           `/delSource`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "EditEngine",
			Router:           `/editEngine`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "EditSource",
			Router:           `/editSource`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "EngineConfig",
			Router:           `/engineConfig`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "Engines",
			Router:           `/engines`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "Sources",
			Router:           `/sources`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "GetRuleStatus",
			Router:           `/status`,
			AllowHTTPMethods: []string{"get"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:EngineController"] = append(beego.GlobalControllerRouter["iotServer/controllers:EngineController"],
		beego.ControllerComments{
			Method:           "Valid",
			Router:           `/valid`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:GroupController"] = append(beego.GlobalControllerRouter["iotServer/controllers:GroupController"],
		beego.ControllerComments{
			Method:           "BatchGroup",
			Router:           `/batchGroup`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:GroupController"] = append(beego.GlobalControllerRouter["iotServer/controllers:GroupController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/delete`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:GroupController"] = append(beego.GlobalControllerRouter["iotServer/controllers:GroupController"],
		beego.ControllerComments{
			Method:           "GroupList",
			Router:           `/list`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:GroupController"] = append(beego.GlobalControllerRouter["iotServer/controllers:GroupController"],
		beego.ControllerComments{
			Method:           "Save",
			Router:           `/save`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:GroupController"] = append(beego.GlobalControllerRouter["iotServer/controllers:GroupController"],
		beego.ControllerComments{
			Method:           "UnBatchGroup",
			Router:           `/unBatchGroup`,
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

	beego.GlobalControllerRouter["iotServer/controllers:ModelController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ModelController"],
		beego.ControllerComments{
			Method:           "Template",
			Router:           `/template`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ModelController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ModelController"],
		beego.ControllerComments{
			Method:           "TypeList",
			Router:           `/typeList`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:PositionController"] = append(beego.GlobalControllerRouter["iotServer/controllers:PositionController"],
		beego.ControllerComments{
			Method:           "Create",
			Router:           `/create`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:PositionController"] = append(beego.GlobalControllerRouter["iotServer/controllers:PositionController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/delete`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:PositionController"] = append(beego.GlobalControllerRouter["iotServer/controllers:PositionController"],
		beego.ControllerComments{
			Method:           "Edit",
			Router:           `/edit`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:PositionController"] = append(beego.GlobalControllerRouter["iotServer/controllers:PositionController"],
		beego.ControllerComments{
			Method:           "List",
			Router:           `/tree`,
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
			Method:           "Detail",
			Router:           `/detail`,
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
			Method:           "SaveUnits",
			Router:           `/saveUnits`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:ProductController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ProductController"],
		beego.ControllerComments{
			Method:           "Units",
			Router:           `/units`,
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

	beego.GlobalControllerRouter["iotServer/controllers:ReportController"] = append(beego.GlobalControllerRouter["iotServer/controllers:ReportController"],
		beego.ControllerComments{
			Method:           "TimePeriodReport",
			Router:           `/timePeriod`,
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
			Method:           "List",
			Router:           `/list`,
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

	beego.GlobalControllerRouter["iotServer/controllers:SceneController"] = append(beego.GlobalControllerRouter["iotServer/controllers:SceneController"],
		beego.ControllerComments{
			Method:           "Edit",
			Router:           `/edit`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:SceneController"] = append(beego.GlobalControllerRouter["iotServer/controllers:SceneController"],
		beego.ControllerComments{
			Method:           "List",
			Router:           `/list`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:SceneController"] = append(beego.GlobalControllerRouter["iotServer/controllers:SceneController"],
		beego.ControllerComments{
			Method:           "OperateScene",
			Router:           `/operate`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:SceneController"] = append(beego.GlobalControllerRouter["iotServer/controllers:SceneController"],
		beego.ControllerComments{
			Method:           "GetSceneStatus",
			Router:           `/status`,
			AllowHTTPMethods: []string{"get"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:SceneController"] = append(beego.GlobalControllerRouter["iotServer/controllers:SceneController"],
		beego.ControllerComments{
			Method:           "Update",
			Router:           `/update`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:TenantController"] = append(beego.GlobalControllerRouter["iotServer/controllers:TenantController"],
		beego.ControllerComments{
			Method:           "Setting",
			Router:           `/setting`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:TenantController"] = append(beego.GlobalControllerRouter["iotServer/controllers:TenantController"],
		beego.ControllerComments{
			Method:           "UploadIcon",
			Router:           `/uploadIcon`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:TenantController"] = append(beego.GlobalControllerRouter["iotServer/controllers:TenantController"],
		beego.ControllerComments{
			Method:           "UploadImage",
			Router:           `/uploadImage`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:TenantController"] = append(beego.GlobalControllerRouter["iotServer/controllers:TenantController"],
		beego.ControllerComments{
			Method:           "UploadLogo",
			Router:           `/uploadLogo`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UdfController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UdfController"],
		beego.ControllerComments{
			Method:           "Create",
			Router:           `/create`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UdfController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UdfController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/delete`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UdfController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UdfController"],
		beego.ControllerComments{
			Method:           "DeleteService",
			Router:           `/deleteService`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UdfController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UdfController"],
		beego.ControllerComments{
			Method:           "EditService",
			Router:           `/editService`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UdfController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UdfController"],
		beego.ControllerComments{
			Method:           "Get",
			Router:           `/get`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UdfController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UdfController"],
		beego.ControllerComments{
			Method:           "ListServices",
			Router:           `/listServices`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UdfController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UdfController"],
		beego.ControllerComments{
			Method:           "Update",
			Router:           `/update`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

	beego.GlobalControllerRouter["iotServer/controllers:UserController"] = append(beego.GlobalControllerRouter["iotServer/controllers:UserController"],
		beego.ControllerComments{
			Method:           "Delete",
			Router:           `/del`,
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
				param.New("deviceCode", param.IsRequired),
				param.New("tagCode", param.IsRequired),
				param.New("val", param.IsRequired),
			),
			Filters: nil,
			Params:  nil})

	beego.GlobalControllerRouter["iotServer/controllers:WriteController"] = append(beego.GlobalControllerRouter["iotServer/controllers:WriteController"],
		beego.ControllerComments{
			Method:           "Log",
			Router:           `/log`,
			AllowHTTPMethods: []string{"post"},
			MethodParams:     param.Make(),
			Filters:          nil,
			Params:           nil})

}
