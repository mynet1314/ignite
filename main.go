package main

import (
	"flag"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mynet1314/nlan/controllers"
	"github.com/mynet1314/nlan/utils"
)

var (
	confPath    = flag.String("c", "./conf/config.toml", "config file")
	versionFlag = flag.Bool("v", false, "version")
	version     = "unknown"
)

func main() {
	flag.Parse()
	if *versionFlag {
		fmt.Println(version)
		return
	}
	utils.InitConf(*confPath)
	initRouter()
}

func initRouter() {
	r := gin.Default()

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	mainRouter := &controllers.MainRouter{}
	mainRouter.Initialize(r)
}
