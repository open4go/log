package model

import (
	"context"
	"fmt"
	"github.com/r2day/db"
)

type DemoModel struct {
	Model
	// 名称
	Name string `json:"name" bson:"name"`
	// 描述
	Desc string `json:"desc" bson:"desc"`
	// 引用次数
	Reference uint `json:"reference" bson:"reference"`
}

func Demo() {
	d := &DemoModel{}
	d.Name = "i am demo"
	d.Desc = "i am desc"

	m := d.Init(context.TODO(), db.MDB, "demo")
	s, err := m.Create(d)
	if err != nil {
		panic(err)
	}
	fmt.Println(s)
}
