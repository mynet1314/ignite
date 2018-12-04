package controllers

import (
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/mynet1314/nlan/models"
	"github.com/mynet1314/nlan/ss"
	"github.com/mynet1314/nlan/utils"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	servers          = []string{"SSR"}
	ssMethods        = []string{"aes-256-cfb", "aes-128-gcm", "aes-192-gcm", "aes-256-gcm", "chacha20-ietf-poly1305"}
	ssrMethods       = []string{"aes-256-cfb", "aes-256-ctr", "chacha20", "chacha20-ietf"}
	serverMethodsMap = map[string]map[string]bool{}
)

func init() {
	ssMethodMap := map[string]bool{}
	for _, method := range ssMethods {
		ssMethodMap[method] = true
	}
	ssrMethodMap := map[string]bool{}
	for _, method := range ssrMethods {
		ssrMethodMap[method] = true
	}

	serverMethodsMap["SS"] = ssMethodMap
	serverMethodsMap["SSR"] = ssrMethodMap
}

func (router *MainRouter) PanelIndexHandler(c *gin.Context) {
	userID, exists := c.Get("userId")

	if !exists {
		c.HTML(http.StatusOK, "panel.html", nil)
		return
	}

	user := new(models.User)
	exists, _ = router.db.Id(userID).Get(user)

	if !exists {
		//Service has been removed by admininistrator.
		session := sessions.Default(c)
		session.Delete("userId")
		session.Save()

		c.Redirect(http.StatusFound, "/")
		return
	}
	if !user.EmailChecked {
		c.Redirect(http.StatusFound, "/panel/email_check")
		return
	}

	uInfo := &models.UserInfo{
		Id:            user.Id,
		Host:          ss.Host,
		Username:      user.Username,
		Status:        user.Status,
		PackageUsed:   fmt.Sprintf("%.2f", user.PackageUsed),
		PackageLimit:  user.PackageLimit,
		PackageLeft:   fmt.Sprintf("%.2f", float32(user.PackageLimit)-user.PackageUsed),
		ServicePort:   user.ServicePort,
		ServicePwd:    user.ServicePwd,
		ServiceMethod: user.ServiceMethod,
		ServiceType:   user.ServiceType,
		Expired:       user.Expired.Format("2006-01-02"),
		ServiceURL:    utils.ServiceURL(user.ServiceType, utils.HOST_Address, user.ServicePort, user.ServiceMethod, user.ServicePwd),
	}
	if uInfo.ServiceMethod == "" {
		uInfo.ServiceMethod = "aes-256-cfb"
	}
	if uInfo.ServiceType == "" {
		uInfo.ServiceType = "SS"
	}

	if user.PackageLimit == 0 {
		uInfo.PackageLeftPercent = "0"
	} else {
		uInfo.PackageLeftPercent = fmt.Sprintf("%.2f", (float32(user.PackageLimit)-user.PackageUsed)/float32(user.PackageLimit)*100)
	}

	c.HTML(http.StatusOK, "panel.html", gin.H{
		"uInfo":       uInfo,
		"ss_methods":  ssMethods,
		"ssr_methods": ssrMethods,
		"servers":     servers,
	})
}

func (router *MainRouter) LogoutHandler(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete("userId")
	session.Save()

	c.Redirect(http.StatusFound, "/")
}

func (router *MainRouter) DonateHandler(c *gin.Context) {
	userID, exists := c.Get("userId")

	if !exists {
		c.HTML(http.StatusOK, "panel.html", nil)
		return
	}

	user := new(models.User)
	exists, _ = router.db.Id(userID).Get(user)

	if !exists {
		//Service has been removed by admininistrator.
		session := sessions.Default(c)
		session.Delete("userId")
		session.Save()

		c.Redirect(http.StatusFound, "/")
		return
	}
	if !user.EmailChecked {
		c.Redirect(http.StatusFound, "/panel/email_check")
		return
	}

	c.HTML(http.StatusOK, "donate.html", gin.H{
		"uInfo": 1,
	})

}

func (router *MainRouter) ActualDonateHandler(c *gin.Context) {
	month, _ := strconv.Atoi(c.PostForm("month"))
	donate_type, _ := strconv.Atoi(c.PostForm("donate_type"))

	if month <= 0 {
		resp := models.Response{Success: false, Message: "请选择一个捐助时间!"}
		c.JSON(http.StatusOK, &resp)
		return
	}
	nickname := c.PostForm("nickname")
	if strings.Trim(nickname, " ") == "" {
		resp := models.Response{Success: false, Message: "请输入微信或支付宝昵称!"}
		c.JSON(http.StatusOK, &resp)
		return
	}
	userID, exists := c.Get("userId")

	if !exists {
		resp := models.Response{Success: false, Message: "后台宝宝找不到你啦，请联系管理员!"}
		c.JSON(http.StatusOK, &resp)
		return
	}

	user := new(models.User)
	exists, _ = router.db.Id(userID).Get(user)

	if !exists {
		//Service has been removed by admininistrator.
		session := sessions.Default(c)
		session.Delete("userId")
		session.Save()

		resp := models.Response{Success: false, Message: "太长时间没操作啦，请重新登录!"}
		c.JSON(http.StatusOK, &resp)
		return
	}

	donate := new(models.Donate)
	//Create user account
	trans := router.db.NewSession()
	defer trans.Close()

	trans.Begin()

	//1.Create user account
	donate.Nickname = nickname
	donate.DonateType = donate_type
	donate.Month = month
	donate.Created = time.Now()

	affected, err := trans.Insert(donate)

	if affected == 0 {
		trans.Rollback()
		fmt.Println("Failed to create donate record!", err)
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "捐赠记录创建失败，请联系管理员"})
		return
	}
	if err := trans.Commit(); err != nil {
		fmt.Println("Failed to create donate record 2!")
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "捐赠记录创建失败，请联系管理员"})
		return
	}

	now := time.Now()
	// 如果服务已经到期了
	if user.Expired.Before(now) {
		// 直接将过期时间加多少天
		user.Expired = now.AddDate(0, month, 0)
	} else {
		// 不然就直接在当前的过期时间上加上一个时间
		user.Expired = user.Expired.AddDate(0, month, 0)
	}

	if user.ServiceId != "" {
		if user.Status == 2 || !ss.IsContainerRunning(user.ServiceId) {
			if err := ss.StartContainer(user.ServiceId); err != nil {
				resp := models.Response{Success: false, Message: "启动服务失败，请联系管理员!"}
				c.JSON(http.StatusOK, &resp)
				return
			}
			user.Status = 1
		}
	}

	if _, err := router.db.Id(userID).Cols("expired", "status").Update(user); err != nil {
		resp := models.Response{Success: false, Message: "更新服务状态失败，请联系管理员!"}
		c.JSON(http.StatusOK, &resp)
		return
	}

	resp := models.Response{Success: true, Message: "success"}
	c.JSON(http.StatusOK, resp)
}

func (router *MainRouter) EmailConfirmHandler(c *gin.Context) {
	// userId := strconv.Atoi(c.Query("id"))
	userid := c.Query("id")
	userId, _ := strconv.Atoi(userid)
	code := c.Query("code")

	user := new(models.User)
	exists, _ := router.db.Id(userId).Get(user)
	// TODO: 这里默认它就是合法的啦
	fmt.Println("UserID...", user.Id, exists)
	if user.EmailCheckCode == code {
		user.EmailChecked = true
		router.db.Id(userId).AllCols().Update(user)
	}
	c.HTML(http.StatusOK, "emailCheckComplete.html", gin.H{
		"uInfo": 1,
	})

}
func (router *MainRouter) EmailCheckHandler(c *gin.Context) {
	userID, _ := c.Get("userId")

	if userID == nil {
		c.HTML(http.StatusOK, "index.html", gin.H{})
		return
	}
	user := new(models.User)
	exists, _ := router.db.Id(userID).Get(user)

	if !exists {
		//Service has been removed by admininistrator.
		session := sessions.Default(c)
		session.Delete("userId")
		session.Save()

		c.HTML(http.StatusOK, "index.html", gin.H{})
		return
	}

	c.HTML(http.StatusOK, "emailCheck.html", gin.H{
		"uInfo": 1,
		"email": user.Email})

}

func (router *MainRouter) CreateServiceHandler(c *gin.Context) {
	userID, _ := c.Get("userId")
	user := new(models.User)
	exists, _ := router.db.Id(userID).Get(user)

	if !exists {
		//Service has been removed by admininistrator.
		session := sessions.Default(c)
		session.Delete("userId")
		session.Save()

		c.Redirect(http.StatusFound, "/")
		return
	}
	if !user.EmailChecked {
		c.Redirect(http.StatusFound, "/panel/email_check")
		return
	}

	method := c.PostForm("method")
	serverType := c.PostForm("server-type")

	fmt.Println("UserID", userID)
	fmt.Println("ServerType:", serverType)
	fmt.Println("Method:", method)

	methodMap, ok := serverMethodsMap[serverType]
	if !ok {
		resp := models.Response{Success: false, Message: "服务类型配置错误!"}
		c.JSON(http.StatusOK, resp)
		return
	}

	if !methodMap[method] {
		resp := models.Response{Success: false, Message: "加密方法配置错误!"}
		c.JSON(http.StatusOK, resp)
		return
	}

	if user.ServiceId != "" {
		resp := models.Response{Success: false, Message: "服务已创建!"}
		c.JSON(http.StatusOK, resp)
		return
	}

	//Get all used ports.
	var usedPorts []int
	router.db.Table("user").Cols("service_port").Find(&usedPorts)

	// 1. Create ss service
	port, err := utils.GetAvailablePort(&usedPorts)
	if err != nil {
		resp := models.Response{Success: false, Message: "创建服务失败,没有可用端口!"}
		c.JSON(http.StatusOK, resp)
		return
	}
	result, err := ss.CreateAndStartContainer(serverType, strings.ToLower(user.Username), method, "", port)
	if err != nil {
		log.Println("Create ss service error:", err.Error())
		resp := models.Response{Success: false, Message: "创建服务失败!"}
		c.JSON(http.StatusOK, resp)
		return
	}

	// 2. Update user info
	user.Status = 1
	user.ServiceId = result.ID
	user.ServicePort = result.Port
	user.ServicePwd = result.Password
	user.ServiceMethod = method
	user.ServiceType = serverType
	affected, err := router.db.Id(userID).Cols("status", "service_port", "service_pwd", "service_id", "service_method", "service_type").Update(user)
	if affected == 0 || err != nil {
		if err != nil {
			log.Println("Update user info error:", err.Error())
		}

		//Force remove created container
		ss.RemoveContainer(result.ID)

		resp := models.Response{Success: false, Message: "更新用户信息失败!"}
		c.JSON(http.StatusOK, resp)
		return
	}

	result.PackageLimit = user.PackageLimit
	result.Host = ss.Host
	resp := models.Response{Success: true, Message: "服务创建成功!", Data: result}

	c.JSON(http.StatusOK, resp)
}
