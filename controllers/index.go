package controllers

import (
	"fmt"
	"bytes"
	"net/http"
	"regexp"
	"time"
	"strings"
	"encoding/hex"
	"crypto/tls"
	"golang.org/x/crypto/bcrypt"
	"crypto/md5"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/mynet1314/nlan/models"
	"gopkg.in/gomail.v2"
	"os"
	"strconv"
	"html/template"
)

func (router *MainRouter) IndexHandler(c *gin.Context) {
	session := sessions.Default(c)
	v := session.Get("userId")
	var userId int64 = 0
	var uInfo *models.UserInfo
	if v != nil {
		if uId, ok := v.(int64); ok {
			uInfo = &models.UserInfo{
				Id: uId,
			}
			userId = uId
		}
	}
	user := new(models.User)
	exists, _ := router.db.Id(userId).Get(user)
	if (exists && !user.EmailChecked) {
		c.Redirect(http.StatusFound, "/panel/email_check")
		return
	}
	c.HTML(http.StatusOK, "index.html", gin.H{
		"uInfo": uInfo,
	})
}

func (router *MainRouter) LoginHandler(c *gin.Context) {
	username := c.PostForm("username")
	pwd := c.PostForm("password")

	user := new(models.User)

	email_matched, _ := regexp.MatchString("^([a-zA-Z0-9_-])+@([a-zA-Z0-9_-])+(.[a-zA-Z0-9_-])+", username)
	if (email_matched) {
		router.db.Where("email = ?", username).Get(user)
	} else {
		router.db.Where("username = ?", username).Get(user)
	}

	if user.Id == 0 {
		fmt.Println("User doesn't exist!")
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "用户名或密码错误!"})
		return
	}

	if bcrypt.CompareHashAndPassword(user.HashedPwd, []byte(pwd)) != nil {
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "用户名或密码错误!"})
		return
	}

	fmt.Println("Come here...", user.EmailChecked)
	fmt.Println("userId is:", user.Id)

	session := sessions.Default(c)
	session.Set("userId", user.Id)
	session.Save()

	sendEmail(user.Id, user.Email, user.Username, user.EmailCheckCode)
	c.JSON(http.StatusOK, &models.Response{Success: true, Message: "Success!"})
}

func MD5(text string) string{
   ctx := md5.New()
   ctx.Write([]byte(text))
   return hex.EncodeToString(ctx.Sum(nil))
}

func (router *MainRouter) ResendEmailHandler(c *gin.Context) {
	session := sessions.Default(c)
	v := session.Get("userId")
	var userId int64 = 0
	if v != nil {
		if uId, ok := v.(int64); ok {
			userId = uId
		}
	} else {
		c.Redirect(http.StatusFound, "/")
		return
	}

	fmt.Println("userID!", userId)

	user := new(models.User)
	exists, _ := router.db.Id(userId).Get(user)

	if !exists {
		//Service has been removed by admininistrator.
		session.Delete("userId")
		session.Save()

		c.Redirect(http.StatusFound, "/")
		return
	}

	email := c.PostForm("email")
	fmt.Println("email!", email)
	if email == "" {
		sendEmail(user.Id, user.Email, user.Username, user.EmailCheckCode)
		c.JSON(http.StatusOK, &models.Response{Success: true, Message: "Success!"})
	} else {
		if !isEmail(email) {
			fmt.Println("Email is invalid!", email)
			c.JSON(http.StatusOK, &models.Response{Success: false, Message: "请输入有效的邮箱"})
			return
		}
		user.Email = email
		affected, err := router.db.Id(userId).Cols("Email").Update(user)
		if affected == 0 || err != nil {
			if err != nil {
				c.JSON(http.StatusOK, &models.Response{Success: false, Message: "更改邮箱失败，请重试或联系管理员"})
				return
			}
		}
		sendEmail(user.Id, user.Email, user.Username, user.EmailCheckCode)
		c.Redirect(http.StatusFound, "/panel/email_check")
	}
}

func sendEmail(userid int64, email string, username string, checkcode string) {
	// TODO: 发邮件放在消息队列里面
	m := gomail.NewMessage()
	from_email := os.Getenv("EMAIL_ACCOUNT")
	from_email_pwd := os.Getenv("EMAIL_PASSWARD")
	email_server := os.Getenv("EMAIL_SERVER")
	email_port, _ := strconv.Atoi(os.Getenv("EMAIL_PORT"))
	m.SetHeader("From", from_email)
	m.SetHeader("To", email, email)
	m.SetHeader("Subject", "邮箱验证!")
	var doc bytes.Buffer
	tmpl, err := template.ParseFiles("templates/email_confirm.html")
	tmpl.Execute(&doc, struct {
		Userid	int64
		Code	string
		Username string
		WebSite string
	}{
		userid,
		checkcode,
		username,
		os.Getenv("WEBSITE"),
	})

	fmt.Println("tmpl!", tmpl, err, doc.String())
	m.SetBody("text/html", doc.String())
	d := gomail.NewDialer(email_server, email_port, from_email, from_email_pwd)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	// Send the email to Bob, Cora and Dan.
	if err := d.DialAndSend(m); err != nil {
	    panic(err)
	}
}

func isEmail(text string) bool {
	email_matched, _ := regexp.MatchString("^([a-zA-Z0-9_-])+@([a-zA-Z0-9_-])+(.[a-zA-Z0-9_-])+", text)
	return email_matched
}

func (router *MainRouter) SignupHandler(c *gin.Context) {
	inviteCode := c.PostForm("invite-code")
	username := c.PostForm("username")
	email := c.PostForm("email")
	pwd := c.PostForm("password")
	confirmPwd := c.PostForm("confirm-password")

	username_len := strings.Count(username,"") - 1

	if username_len < 3 || username_len > 10 {
		fmt.Println("Username is invalid!", username)
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "用户名需要最少3个最多10个字母或汉字"})
		return
	}

	// TODO: 更正规的邮箱正则表达式验证
	email_matched := isEmail(email)

	if !email_matched {
		fmt.Println("Email is invalid!", email)
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "请输入有效的邮箱"})
		return
	}

	if pwd != confirmPwd {
		fmt.Println("passwords not match!")
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "两次输入的密码不一致"})
		return
	}

	// TODO: 密码不能是中文
	pwd_len := strings.Count(pwd, "") - 1
	if pwd_len < 6 || pwd_len > 20 {
		fmt.Println("passwd is invalid!", pwd)
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "密码长度至少6位"})
		return
	}

	user := new(models.User)
	count, _ := router.db.Where("username = ?", username).Count(user)

	if count > 0 {
		fmt.Println("Username duplicated!")
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "用户名已存在"})
		return
	}

	iv := new(models.InviteCode)
	router.db.Where("invite_code = ? AND available = 1", inviteCode).Get(iv)

	fmt.Println("gen checkcode", MD5(username + email))
	if iv.Id == 0 {
		fmt.Println("Invalid invite code!")
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "邀请码无效"})
		return
	}

	//Create user account
	trans := router.db.NewSession()
	defer trans.Close()

	trans.Begin()

	//1.Create user account
	user.Username = username
	user.Email = email
	user.EmailChecked = false
	user.EmailCheckCode = MD5(username + email)
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	user.HashedPwd = hashedPass
	user.InviteCode = iv.InviteCode
	user.PackageLimit = iv.PackageLimit
	user.Expired = time.Now().AddDate(0, iv.AvailableLimit, 0)

	affected, _ := trans.Insert(user)

	if affected == 0 {
		trans.Rollback()
		fmt.Println("Failed to create user account!")
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "创建用户失败，请联系管理员"})
		return
	}

	//2.Set invite code as used status
	iv.Available = false
	affected, _ = trans.Id(iv.Id).Cols("available").Update(iv)

	if affected == 0 {
		trans.Rollback()
		fmt.Println("Failed to create user account!")
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "创建用户失败，请联系管理员"})
		return
	}

	if err := trans.Commit(); err != nil {
		fmt.Println("Failed to create user account!")
		c.JSON(http.StatusOK, &models.Response{Success: false, Message: "创建用户失败，请联系管理员"})
		return
	}

	session := sessions.Default(c)
	session.Set("userId", user.Id)
	session.Save()

	sendEmail(user.Id, user.Email, user.Username, user.EmailCheckCode)
	fmt.Printf("User %s with invite code: %s", username, inviteCode)
	c.JSON(http.StatusOK, &models.Response{Success: true, Message: "Success!"})
}
