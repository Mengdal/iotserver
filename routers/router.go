package routers

import (
	beego "github.com/beego/beego/v2/server/web"
	"iotServer/controllers"
	"iotServer/controllers/edgeController"
)

func init() {
	ns := beego.NewNamespace("/api",
		beego.NSNamespace("/user",
			beego.NSInclude(
				&controllers.UserController{},
			),
		),
		beego.NSNamespace("/role",
			beego.NSInclude(
				&controllers.RoleController{},
			),
		),
		beego.NSNamespace("/menu",
			beego.NSInclude(
				&controllers.MenuController{},
			),
		),
		beego.NSNamespace("/scada",
			beego.NSInclude(
				&edgeController.HistoryController{},
				&edgeController.RealController{},
				&edgeController.TagController{},
			),
		),
		beego.NSNamespace("/ws",
			beego.NSInclude(
				&edgeController.WebsocketController{},
			),
		),
		beego.NSNamespace("/product",
			beego.NSInclude(
				&controllers.ProductController{},
			),
		),
		beego.NSNamespace("/model",
			beego.NSInclude(
				&controllers.ModelController{},
			),
		),
		beego.NSNamespace("/write",
			beego.NSInclude(
				&controllers.WriteController{},
			),
		),
		beego.NSNamespace("/label",
			beego.NSInclude(
				&edgeController.LabelController{},
			),
		),
	)
	beego.AddNamespace(ns)
}
