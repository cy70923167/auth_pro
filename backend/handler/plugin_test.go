package handler

import "testing"

func TestListedCatalogPluginsIncludeKuaitongAndTencent(t *testing.T) {
	var kuaitong, tencent pluginInfo
	for _, plugin := range listedCatalogPlugins() {
		switch plugin.ID {
		case "kuaitong-realname":
			kuaitong = plugin
		case "tencent-realname":
			tencent = plugin
		}
	}

	if kuaitong.ID == "" {
		t.Fatal("kuaitong realname plugin is missing")
	}
	if kuaitong.Hidden {
		t.Fatal("kuaitong realname plugin is still hidden")
	}
	if kuaitong.Description != "支持姓名与身份证二要素核验，也可扫码拍照完成人脸认证" {
		t.Fatalf("unexpected kuaitong description: %q", kuaitong.Description)
	}

	if tencent.ID == "" {
		t.Fatal("tencent realname plugin is missing")
	}
	if tencent.Name != "靓仔聚合认证" {
		t.Fatalf("unexpected name: %q", tencent.Name)
	}
	if tencent.Description != "靓仔聚合实名认证服务:接入地址为:http://real.4775.cn/" {
		t.Fatalf("unexpected description: %q", tencent.Description)
	}
	if tencent.Homepage != "http://real.4775.cn/" {
		t.Fatalf("unexpected homepage: %q", tencent.Homepage)
	}
	if tencent.Icon != "ri:id-card-line" {
		t.Fatalf("unexpected icon: %q", tencent.Icon)
	}
}
