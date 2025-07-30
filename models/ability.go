package models

import (
	"time"
)

type InPutParams []InputOutput
type OutPutParams []InputOutput //输入参数
type InputOutput struct {
	Code     string `json:"code"`      //eg：CurrentTemperature
	Name     string `json:"name"`      //温度
	TypeSpec string `json:"type_spec"` //{"type":"float","specs":"{\"min\":\"-40\",\"max\":\"120\",\"step\":\"0.01\",\"unit\":\"℃\",\"unitName\":\"摄氏度\"}"}
}

// Properties 属性
type Properties struct {
	Id          int64  `orm:"auto;pk;size(255)" json:"id"`
	AccessMode  string `orm:"size(50);null" json:"access_mode"`
	Name        string `orm:"size(255);null" json:"name"`
	Code        string `orm:"size(255);null" json:"code"`
	Description string `orm:"type(text);null" json:"description"`
	Require     bool   `orm:"default(false);null" json:"require"`
	TypeSpec    string `orm:"column(type_spec);type(text);null" json:"type_spec"`
	Tag         string `orm:"size(50);null" json:"tag"` //系统内置
	System      bool   `orm:"default(false);null" json:"system"`
	Created     int64  `orm:"type(bigint);null" json:"created"` // 时间戳存储
	Updated     int64  `orm:"type(bigint);null" json:"updated"` // 时间戳存储

	Product *Product `orm:"rel(fk);column(product_id);on_delete(cascade)" json:"-"`
}

// Events 事件
type Events struct {
	Id           int64  `orm:"auto;pk;size(255)" json:"id"`
	EventType    string `orm:"size(255);null" json:"event_type"`
	Code         string `orm:"size(255);null;unique" json:"code"`
	Name         string `orm:"size(255);null" json:"name"`
	Description  string `orm:"type(text);null" json:"description"`
	Require      bool   `orm:"default(false);null" json:"require"`
	OutputParams string `orm:"column(output_params);type(text);null" json:"output_params"`
	Tag          string `orm:"size(50);null" json:"tag"`
	System       bool   `orm:"default(false);null" json:"system"`
	Created      int64  `orm:"type(bigint);null" json:"created"` // 时间戳存储
	Updated      int64  `orm:"type(bigint);null" json:"updated"` // 时间戳存储

	Product *Product `orm:"rel(fk);column(product_id);on_delete(cascade)" json:"-"`
}

// Actions 服务
type Actions struct {
	Id           int64  `orm:"auto;pk;size(255)" json:"id"`
	CallType     string `orm:"size(50);null" json:"call_type"`
	Code         string `orm:"size(255);null;unique" json:"code"`
	Name         string `orm:"size(255);null" json:"name"`
	Description  string `orm:"type(text);null" json:"description"`
	Require      bool   `orm:"default(false);null" json:"require"`
	InputParams  string `orm:"column(input_params);type(text);null" json:"input_params"`
	OutputParams string `orm:"column(output_params);type(text);null" json:"output_params"`
	Tag          string `orm:"size(50);null" json:"tag"`
	System       bool   `orm:"default(false);null" json:"system"`
	Created      int64  `orm:"type(bigint);null" json:"created"` // 时间戳存储
	Updated      int64  `orm:"type(bigint);null" json:"updated"` // 时间戳存储

	Product *Product `orm:"rel(fk);column(product_id);on_delete(cascade)" json:"-"`
}

func (p *Properties) BeforeInsert() error {
	now := time.Now().Unix()
	if p.Created == 0 {
		p.Created = now
	}
	p.Updated = now
	return nil
}

func (p *Properties) BeforeUpdate() error {
	p.Updated = time.Now().Unix()
	return nil
}

func (p *Events) BeforeInsert() error {
	now := time.Now().Unix()
	if p.Created == 0 {
		p.Created = now
	}
	p.Updated = now
	return nil
}

func (p *Events) BeforeUpdate() error {
	p.Updated = time.Now().Unix()
	return nil
}

func (p *Actions) BeforeInsert() error {
	now := time.Now().Unix()
	if p.Created == 0 {
		p.Created = now
	}
	p.Updated = now
	return nil
}

func (p *Actions) BeforeUpdate() error {
	p.Updated = time.Now().Unix()
	return nil
}
