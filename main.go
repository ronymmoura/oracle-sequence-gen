package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	_ "github.com/sijms/go-ora/v2"
)

func main() {
	var host string
	var serviceName string
	var port string = "1521"
	var user string
	var password string
	var tablesFilter string
	var pkPrefix string
	var sequencePrefix string = "S_"

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Host").Value(&host),
			huh.NewInput().Title("Port").Value(&port),
			huh.NewInput().Title("Service Name").Value(&serviceName),
			huh.NewInput().Title("Username").Value(&user),
			huh.NewInput().Title("Password").Value(&password),
			huh.NewInput().Title("Tables Filter").Description("Filter tables using regular expressions").Value(&tablesFilter),
			huh.NewInput().Title("PK Prefix").Value(&pkPrefix),
			huh.NewInput().Title("Sequence Prefix").Value(&sequencePrefix),
		),
	)

	err := form.Run()
	if err != nil {
		log.Fatalf("error in form: %v", err)
	}

	connString := fmt.Sprintf("oracle://%s:%s@%s:%s/%s", url.PathEscape(user), url.PathEscape(password), host, port, serviceName)
	fmt.Println(connString)
	db, err := sql.Open("oracle", connString)
	if err != nil {
		log.Fatalf("error in sql.Open: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("error pinging db: %v", err)
	}

	tableNames, err := getTables(db, tablesFilter)
	if err != nil {
		log.Fatalf("error getting tables: %v", err)
	}

	err = dropSequences(db, tableNames, sequencePrefix)
	if err != nil {
		log.Fatalf("error dropping sequences: %v", err)
	}

	err = createSequences(db, tableNames, sequencePrefix, pkPrefix)
	if err != nil {
		log.Fatalf("error creating sequences: %v", err)
	}
}

func getTables(db *sql.DB, tablesFilter string) ([]string, error) {
	rows, err := db.Query("SELECT table_name FROM USER_TABLES order by table_name")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	tableNames := []string{}

	for rows.Next() {
		var tableName string
		err := rows.Scan(&tableName)
		if err != nil {
			return nil, err
		}

		re := regexp.MustCompile(tablesFilter)
		if re.MatchString(tableName) {
			tableNames = append(tableNames, tableName)
		}
	}

	return tableNames, nil
}

func dropSequences(db *sql.DB, tables []string, sequencePrefix string) error {
	dropsBaseQuery := `DROP SEQUENCE %s%s`

	for _, table := range tables {
		dropQuery := fmt.Sprintf(dropsBaseQuery, sequencePrefix, table)
		_, err := db.Exec(dropQuery)
		if err != nil {
			if !strings.Contains(err.Error(), "sequence does not exist") {
				return err
			}
			continue
		}
	}

	return nil
}

func createSequences(db *sql.DB, tables []string, sequencePrefix string, pkPrefix string) error {
	maxBaseQuery := "select max(%s%s) max_key from %s"
	createSequenceBaseQuery := `CREATE SEQUENCE %s%s INCREMENT BY 1 START WITH %d MAXVALUE 1E27 MINVALUE 0 NOCYCLE`

	for i := range tables {
		table := tables[i]
		//fmt.Println(table)
		maxQuery := fmt.Sprintf(maxBaseQuery, pkPrefix, table[len(pkPrefix):], table)
		rows, err := db.Query(maxQuery)
		if err != nil {
			if !strings.Contains(err.Error(), "invalid identifier") {
				return err
			}
			continue
		}
		defer rows.Close()

		var max *int
		rows.Next()
		err = rows.Scan(&max)
		if err != nil {
			return err
		}

		if max != nil {
			createSequenceQuery := fmt.Sprintf(createSequenceBaseQuery, sequencePrefix, table, *max)
			_, err = db.Exec(createSequenceQuery)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
