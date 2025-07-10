package edgeController

import (
	"iotServer/controllers"
)

type MainController struct {
	controllers.BaseController
}

func (c *MainController) Get() {
	c.TplName = "static/dist/index.html"
}
