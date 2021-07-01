package main

import (
	"fmt"

	"github.com/gidyon/micro/pkg/conn"
	"github.com/gidyon/micro/utils/errs"
)

func main() {
	// Open sql connection
	conn, err := conn.OpenGormConn(&conn.DBOptions{
		Dialect:  "msql",
		Address:  "ec2-13-58-199-243.us-east-2.compute.amazonaws.com",
		User:     "onfon",
		Password: "c#7z(9T*s,Z?5SP~)`Jup&E?+LN8NaVB",
		Schema:   "mawingu-portal",
		ConnPool: &conn.DBConnPoolOptions{},
	})
	errs.Panic(err)

	fmt.Println("done")
}
