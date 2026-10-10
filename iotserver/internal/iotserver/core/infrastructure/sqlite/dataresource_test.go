package sqlite

import (
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

func TestDataResourceCRUD(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 新增
	created, err := c.AddDataResource(models.DataResource{
		Name:   "推送实例A",
		Type:   "HTTP推送",
		Option: models.DataResourceOption{"url": "http://127.0.0.1:1/x", "method": "post"},
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if created.Id == "" || created.Created == 0 || created.Modified == 0 {
		t.Fatalf("新增返回异常: %+v", created)
	}
	if created.Health != nil {
		t.Fatalf("未验证时 health 应为 nil, got %v", *created.Health)
	}

	// 同类型重名
	if _, err = c.AddDataResource(models.DataResource{
		Name: "推送实例A", Type: "HTTP推送",
	}); err == nil {
		t.Fatal("同类型重名应报错")
	}
	// 不同类型同名允许
	if _, err = c.AddDataResource(models.DataResource{
		Name: "推送实例A", Type: "InfluxDB",
		Option: models.DataResourceOption{"addr": "127.0.0.1:8086"},
	}); err != nil {
		t.Fatalf("不同类型同名应允许: %v", err)
	}

	// 按 id 查
	got, err := c.DataResourceById(created.Id)
	if err != nil {
		t.Fatalf("byId: %v", err)
	}
	if got.Name != "推送实例A" || got.Option["method"] != "post" {
		t.Fatalf("byId 数据异常: %+v", got)
	}
	if _, err = c.DataResourceById("not-exist"); err == nil {
		t.Fatal("查不存在的资源应报错")
	}

	// 列表：类型过滤 + 分页
	list, total, err := c.DataResourceSearch(0, 10, dtos.DataResourceSearchRequest{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("全部列表 total=%d len=%d, want 2/2", total, len(list))
	}
	list, total, err = c.DataResourceSearch(0, 10, dtos.DataResourceSearchRequest{Type: "HTTP推送"})
	if err != nil {
		t.Fatalf("search by type: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].Type != "HTTP推送" {
		t.Fatalf("类型过滤异常: total=%d len=%d", total, len(list))
	}
	list, total, err = c.DataResourceSearch(0, 1, dtos.DataResourceSearchRequest{})
	if err != nil {
		t.Fatalf("search paged: %v", err)
	}
	if total != 2 || len(list) != 1 {
		t.Fatalf("分页异常: total=%d len=%d, want 2/1", total, len(list))
	}
	list, total, err = c.DataResourceSearch(5, 10, dtos.DataResourceSearchRequest{})
	if err != nil {
		t.Fatalf("search offset: %v", err)
	}
	if total != 2 || len(list) != 0 {
		t.Fatalf("越界分页异常: total=%d len=%d, want 2/0", total, len(list))
	}

	// 修改：名称/参数更新，created 不变
	if err = c.UpdateDataResource(models.DataResource{
		Id: created.Id, Name: "推送实例B", Type: "HTTP推送",
		Option: models.DataResourceOption{"url": "http://127.0.0.1:2/y"},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, _ := c.DataResourceById(created.Id)
	if updated.Name != "推送实例B" || updated.Option["url"] != "http://127.0.0.1:2/y" {
		t.Fatalf("更新未生效: %+v", updated)
	}
	if updated.Created != created.Created {
		t.Fatalf("created 被改动: %d -> %d", created.Created, updated.Created)
	}
	if updated.Modified < created.Modified {
		t.Fatalf("modified 未更新: %d", updated.Modified)
	}
	// 改成已存在的同类型名称 → 报错
	if err = c.UpdateDataResource(models.DataResource{
		Id: updated.Id, Name: "推送实例A", Type: "HTTP推送",
	}); err != nil {
		t.Fatalf("改成自己原来的名字应允许: %v", err)
	}

	// 验证结果写回
	if err = c.UpdateDataResourceHealth(created.Id, true); err != nil {
		t.Fatalf("update health: %v", err)
	}
	health, _ := c.DataResourceById(created.Id)
	if health.Health == nil || !*health.Health {
		t.Fatalf("health 未写回: %+v", health.Health)
	}
	if err = c.UpdateDataResourceHealth(created.Id, false); err != nil {
		t.Fatalf("update health false: %v", err)
	}
	health, _ = c.DataResourceById(created.Id)
	if health.Health == nil || *health.Health {
		t.Fatalf("health=false 未写回: %+v", health.Health)
	}

	// 删除
	if err = c.DeleteDataResource(created.Id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err = c.DataResourceById(created.Id); err == nil {
		t.Fatal("删除后仍能查到")
	}
	if err = c.DeleteDataResource(created.Id); err == nil {
		t.Fatal("删除不存在的资源应报错")
	}
}
