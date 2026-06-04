package pg

import (
	"context"
	"net"
	"net/url"
	"openxhh/config"
	"openxhh/loger"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var Conn *pgxpool.Pool

func InitPostgreSQL() {
	UserName := config.ConfigStruct.DataBase.User
	Passwd := config.ConfigStruct.DataBase.Passwd
	Host := config.ConfigStruct.DataBase.Host
	Port := config.ConfigStruct.DataBase.Port
	Db := config.ConfigStruct.DataBase.Db
	ConnStr := buildPostgresConnString(UserName, Passwd, Host, Port, Db)
	var err error
	Conn, err = pgxpool.New(context.Background(), ConnStr)
	if err != nil {
		loger.Loger.Fatal("[DB]Failed to Connect Database", zap.Error(err))
	}
	err = Conn.Ping(context.Background())
	if err != nil {
		loger.Loger.Fatal("[DB]Fatal Error.", zap.Error(err))
	}
	loger.Loger.Info("[DB]PgSQL is OK!")
}

func buildPostgresConnString(userName, passwd, host, port, db string) string {
	hostPort := host
	if port != "" {
		hostPort = net.JoinHostPort(host, port)
	}
	u := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(userName, passwd),
		Host:   hostPort,
		Path:   "/" + db,
	}
	u.RawPath = "/" + url.PathEscape(db)
	return u.String()
}
