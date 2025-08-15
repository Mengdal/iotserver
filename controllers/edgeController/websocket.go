package edgeController

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/gorilla/websocket"
	"iotServer/controllers"
	"iotServer/iotp"
	"iotServer/models"
	"iotServer/services"
	"iotServer/utils"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type WebsocketController struct {
	controllers.BaseController
}

type operate struct {
	Type  string   `json:"type"`
	Id    string   `json:"id"`
	Ids   []string `json:"ids"`
	Val   string   `json:"val"`
	Token string   `json:"token"`
}

var mx sync.Mutex
var Clients = make(map[*websocket.Conn]bool)
var upgrader = websocket.Upgrader{
	// cross origin domain
	CheckOrigin: func(r *http.Request) bool {
		return true // 为true时表示支持websocket跨域
	},
}

func ReadWsMsg(ws *websocket.Conn) {
	log.Println("开始连接")
	mx.Lock()
	Clients[ws] = true
	mx.Unlock()
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			fmt.Println(fmt.Sprintf("页面可能断开啦：%v", err.Error()))
			mx.Lock()
			delete(Clients, ws)
			mx.Unlock()
			break
		}
		fmt.Println(string(message))
		var o operate
		err = json.Unmarshal(message, &o)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			if o.Type == "read" {
				log.Println("读取数据")
				go getRealData(o.Ids, ws)
			} else if o.Type == "write" {
				log.Println("写入数据")
				go writeData(o.Id, ws, o.Token, o.Val)
			}
		}
	}
}

// 4. 返回响应
type response struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
	Val     string      `json:"val"`
}

func writeData(id string, ws *websocket.Conn, token string, val string) error {
	// 1. Token验证
	claims, ok := utils.ParseToken(token)
	if !ok {
		//TODO 后面加入token校验
		log.Println("token过期")
		//return fmt.Errorf("token过期")
	}

	// 2. 获取用户ID
	userId, ok := claims["user_id"].(float64)
	if !ok {
		//TODO 后面加入token校验
		userId = 0
		//return fmt.Errorf("无效的用户ID")
	}

	// 3. 解析设备标签
	parts := strings.Split(id, ".")
	if len(parts) != 2 {
		return fmt.Errorf("无效格式")
	}
	deviceCode, tagCode := parts[0], parts[1]

	// 4. 准备响应模板
	o := orm.NewOrm()
	success := response{Code: 200, Message: "写入成功", Val: val}
	fail := response{Code: 400, Message: "写入失败", Val: val}

	// 5. 执行控制命令
	mx.Lock()
	defer mx.Unlock()
	if _, ok := Clients[ws]; !ok {
		return fmt.Errorf("连接已断开")
	}

	var tagID = deviceCode + "." + tagCode
	// 发送控制命令
	seq, err := services.Processor.Deal(deviceCode, tagCode, val, "组态下发", int64(userId))
	if err != nil {
		fail.Data = tagID
		ws.WriteJSON(fail)
		return nil
	}

	// 检查执行结果
	time.Sleep(time.Second)
	var logs models.WriteLog
	if err := o.QueryTable(new(models.WriteLog)).Filter("seq", seq).One(&logs); err != nil {
		fail.Data = tagID
		fail.Message = "写入失败"
		ws.WriteJSON(fail)
	} else if logs.Status != "SUCCESS" {
		fail.Data = tagID
		fail.Message = "写入超时"
		log.Println("写入超时")
		ws.WriteJSON(fail)
	} else {
		success.Data = tagID
		ws.WriteJSON(success)
	}

	return nil
}

func getRealData(ids []string, ws *websocket.Conn) {
	log.Println("订阅数据")
	device2tag := make(map[string][]string)
	for _, tagId := range ids {
		l := strings.Split(tagId, ".")
		if len(l) == 2 {
			deviceCode := l[0]
			tagCode := l[1]
			if _, ok := device2tag[deviceCode]; !ok {
				device2tag[deviceCode] = make([]string, 0)
			}
			device2tag[deviceCode] = append(device2tag[deviceCode], tagCode)
		}
	}

	for {
		mx.Lock()
		if _, ok := Clients[ws]; !ok {
			mx.Unlock()
			return
		}
		mx.Unlock()
		data, err := iotp.GetRealData(device2tag)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			type response struct {
				Code    int         `json:"code"`
				Data    interface{} `json:"data"`
				Message string      `json:"message"`
			}
			res := response{Code: 200, Data: data}
			records, _ := json.Marshal(res)
			ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			ws.WriteMessage(websocket.TextMessage, records)
		}
		time.Sleep(time.Second)
	}
}

// Get @Title WebSocket连接
// @Description 建立WebSocket连接用于设备实时数据推送
// @Success 101 {string} string "Switching Protocols (WebSocket连接升级成功)"
// @Failure 400 {object} controllers.ErrorResponse "Token无效或参数错误"
// @Failure 500 {object} controllers.ErrorResponse "服务器内部错误"
// @router / [get]
func (c *WebsocketController) Get() {
	ws, err := upgrader.Upgrade(c.Ctx.ResponseWriter, c.Ctx.Request, nil)
	if err != nil {
		fmt.Println(fmt.Sprintf("Get转换成websocket失败：%v", err.Error()))
		c.Data["json"] = map[string]interface{}{"result": false}
		c.ServeJSON()
		return
	}

	ReadWsMsg(ws)
	c.Data["json"] = map[string]interface{}{"result": false}
	c.ServeJSON()
}
