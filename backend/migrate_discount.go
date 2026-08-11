//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := os.Getenv("AUTH_PRO_MIGRATE_DSN")
	if dsn == "" {
		log.Fatal("请先设置 AUTH_PRO_MIGRATE_DSN 数据库连接串")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("连接失败:", err)
	}
	defer db.Close()

	migrations := []string{
		"ALTER TABLE roles ADD COLUMN discount DECIMAL(3,1) DEFAULT 10.0 COMMENT '折扣(1-10, 10=无折扣)' AFTER description",
		"UPDATE roles SET discount = 10.0 WHERE role_code = 'R_SUPER'",
		"INSERT IGNORE INTO roles (id, role_name, role_code, description, discount, enabled) VALUES (2, '代理商', 'R_AGENT', '代理商角色，可管理下级授权', 8.0, 1), (3, '服务商', 'R_SERVICE', '服务商角色，提供技术服务', 7.0, 1), (4, '合作商', 'R_PARTNER', '合作商角色，合作推广', 6.5, 1)",
		"INSERT IGNORE INTO role_menus (role_id, menu_id) SELECT 2, id FROM menus WHERE name NOT LIKE 'System%' AND name NOT LIKE 'Menu%'",
		"INSERT IGNORE INTO role_menus (role_id, menu_id) SELECT 3, id FROM menus WHERE name NOT LIKE 'System%' AND name NOT LIKE 'Menu%'",
		"INSERT IGNORE INTO role_menus (role_id, menu_id) SELECT 4, id FROM menus WHERE name NOT LIKE 'System%' AND name NOT LIKE 'Menu%'",
	}

	for i, sql := range migrations {
		_, err := db.Exec(sql)
		if err != nil {
			fmt.Printf("[%d] 跳过(可能已存在): %s\n    错误: %v\n", i+1, sql[:min(len(sql), 60)], err)
		} else {
			fmt.Printf("[%d] 成功: %s\n", i+1, sql[:min(len(sql), 60)])
		}
	}

	rows, _ := db.Query("SELECT id, role_name, role_code, discount FROM roles")
	if rows != nil {
		defer rows.Close()
		fmt.Println("\n当前角色列表:")
		for rows.Next() {
			var id int
			var name, code string
			var discount float64
			rows.Scan(&id, &name, &code, &discount)
			fmt.Printf("  ID=%d  %s (%s)  折扣=%.1f\n", id, name, code, discount)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
