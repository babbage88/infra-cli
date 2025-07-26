package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*
var embedMigrations embed.FS

type DbInfo struct {
	DbName        string `json:"dbName" yaml:"db_name" db:"DbName"`
	DbUser        string `json:"dbUser" yaml:"db_user" db:"DbUser"`
	ServerAddress string `json:"serverAddress" yaml:"server_address" db:"ServerAddress"`
	ServerPort    uint16 `json:"serverPort" yaml:"server_port" db:"ServerPort"`
	ClientAddress string `json:"clientAddress" yaml:"client_address" db:"ClientAddress"`
	ClientPort    uint16 `json:"clientPort" yaml:"client_port" db:"ClientPort"`
}

func (db *DbInfo) ServerPortString() string {
	srvPort := fmt.Sprintf("%d", db.ServerPort)
	return srvPort
}

func (db *DbInfo) ClientPortString() string {
	clientPort := fmt.Sprintf("%d", db.ClientPort)
	return clientPort
}

func getDbInfo(connection *sql.DB) (*DbInfo, error) {
	dbInfo := &DbInfo{}
	var getDbNameQuery = `SELECT current_database() as "DbName", 
	session_user AS "DbUser", 
	inet_server_addr()::text as "ServerAddress", 
	inet_server_port() as "ServerPort", 
	inet_client_addr()::text "ClientAddress", 
	inet_client_port() ClientPort;`
	row := connection.QueryRow(getDbNameQuery)
	err := row.Scan(&dbInfo.DbName, &dbInfo.DbUser, &dbInfo.ServerAddress, &dbInfo.ServerPort, &dbInfo.ClientAddress, &dbInfo.ClientPort)
	if err != nil {
		slog.Error("Error retrieving DbName and DbUser from database", slog.String("error", err.Error()))
		return dbInfo, err
	}
	return dbInfo, err
}

func main() {
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("Error Initializing db", slog.String("error", err.Error()))
	}

	dbInfo, err := getDbInfo(db)
	if err != nil {
		slog.Error("Error retrieving db info", slog.String("error", err.Error()))
	}
	slog.Info("Starting migratioms",
		slog.String("DbName", dbInfo.DbName),
		slog.String("DbUser", dbInfo.DbUser),
		slog.String("ServerAddress", dbInfo.ServerAddress),
		slog.String("ServerPort", dbInfo.ServerPortString()),
		slog.String("ClientAddress", dbInfo.ClientAddress),
	)

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("pgx"); err != nil {
		slog.Error("Error configuring driver for migration", slog.String("error", err.Error()))
	}

	if err := goose.Up(db, "migrations"); err != nil {
		slog.Error("Error configuring driver for migration", slog.String("error", err.Error()))
	}
}
