package utils

import (
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mynet1314/nlan/models"
)

func InitDB(driver, connect string) *xorm.Engine {
	//Init DB connection
	switch driver {
	case "mysql", "sqlite3":
	default:
		log.Fatalln("Wrong db driver name:", driver)
	}
	engine, err := xorm.NewEngine(driver, connect)
	if err != nil {
		log.Fatalln("New db engine error:", err.Error())
	}

	err = engine.Ping()
	if err != nil {
		log.Fatalln("Cannot connetc to database:", err.Error())
	}

	err = engine.Sync2(new(models.User), new(models.InviteCode), new(models.Donate))
	if err != nil {
		log.Fatalln("Failed to sync database struct:", err.Error())
	}
	return engine
}
