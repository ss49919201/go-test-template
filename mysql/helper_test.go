package test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/ory/dockertest/v3"
)

func TestMain(m *testing.M) {
	shutdown, err := setup()
	if err != nil {
		panic(err)
	}
	// panic対策
	defer func() {
		os.Stdout.WriteString("Shutdown...\n")
		shutdown()
	}()
	m.Run()
}

func setup() (func(), error) {
	// 環境変数
	_ = godotenv.Load("../.env")

	// Docker pool
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, err
	}

	// マウント対象のファイルが配置されるパスを取得
	dir, _ := os.Getwd()
	for ; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		_, err := os.Stat(filepath.Join(dir, "fixtures"))
		if err == nil {
			break
		}
	}

	// Run
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       "mysql-test",
		Repository: "mysql",
		Tag:        "5.7",
		Env: []string{
			"MYSQL_ROOT_PASSWORD=password",
		},
		Mounts: []string{
			// dir + "/fixtures/mysql:/opt/mysql:ro",
			// dir + "/docker/mysql/conf.d/mysql.cnf:/etc/mysql/conf.d/mysql.cnf:ro",
		},
		Cmd: []string{
			"mysqld", "--character-set-server=utf8mb4", "--collation-server=utf8mb4_unicode_ci",
		},
	})
	if err != nil {
		return nil, err
	}
	os.Setenv("MYSQL_HOST", resource.GetHostPort("3306/tcp"))
	shutdown := func() {
		_ = resource.Close()
	}

	// リクエストを捌く準備ができるまで待つ
	ping := func() error {
		db, err := sql.Open(
			"mysql",
			(&mysql.Config{
				Net:                  "tcp",
				User:                 "root",
				Passwd:               "password",
				DBName:               "mysql",
				Addr:                 os.Getenv("MYSQL_HOST"),
				AllowNativePasswords: true,
			}).FormatDSN(),
		)
		if err != nil {
			return err
		}
		return db.Ping()
	}
	if err = pool.Retry(ping); err != nil {
		shutdown()
		return nil, err
	}

	// 初期化スクリプト
	if _, err := resource.Exec([]string{"sh", "/opt/mysql/setup.sh"}, dockertest.ExecOptions{}); err != nil {
		return nil, err
	}
	// スクリプト終了待ち
	time.Sleep(time.Second * 1)

	return shutdown, nil
}
