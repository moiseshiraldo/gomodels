package gomodel

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

type dbMocker struct {
	queries   []Query
	err       error
	resultErr error
}

func (db *dbMocker) Reset() {
	db.queries = make([]Query, 0)
	db.err = nil
	db.resultErr = nil
}

func (db *dbMocker) Begin() (*sql.Tx, error) {
	return nil, db.err
}

func (db *dbMocker) Close() error {
	return db.err
}

func (db *dbMocker) Commit() error {
	return db.err
}
func (db *dbMocker) Rollback() error {
	return db.err
}

func (db *dbMocker) Exec(stmt string, args ...interface{}) (sql.Result, error) {
	db.queries = append(db.queries, Query{stmt, args})
	return resultMocker{db.resultErr}, db.err
}

func (db *dbMocker) Query(stmt string, args ...interface{}) (*sql.Rows, error) {
	db.queries = append(db.queries, Query{stmt, args})
	return nil, db.err
}

func (db *dbMocker) QueryRow(stmt string, args ...interface{}) *sql.Row {
	db.queries = append(db.queries, Query{stmt, args})
	return &sql.Row{}
}

type resultMocker struct {
	err error
}

func (res resultMocker) LastInsertId() (int64, error) {
	return 42, res.err
}

func (res resultMocker) RowsAffected() (int64, error) {
	return 1, res.err
}

// TestSqliteEngine tests the SqliteEngine methods.
func TestSqliteEngine(t *testing.T) {
	model := &Model{
		name: "User",
		pk:   "id",
		fields: Fields{
			"id":      IntegerField{Auto: true},
			"email":   CharField{MaxLength: 100},
			"active":  BooleanField{DefaultFalse: true},
			"updated": DateTimeField{AutoNow: true},
		},
		meta: Options{
			Table:   "users_user",
			Indexes: Indexes{"test_index": []string{"email"}},
		},
	}
	mockedDB := &dbMocker{}
	engine := SqliteEngine{baseSQLEngine{
		db:          mockedDB,
		driver:      "sqlite3",
		escapeChar:  "\"",
		pHolderChar: "?",
		operators:   sqliteOperators,
	}}
	origScanRow := scanRow
	origOpenDB := openDB
	defer func() {
		scanRow = origScanRow
		openDB = origOpenDB
	}()
	scanRow = func(ex sqlExecutor, dest interface{}, query Query) error {
		db := ex.(*dbMocker)
		db.queries = append(db.queries, query)
		return nil
	}

	t.Run("StartErr", func(t *testing.T) {
		openDB = func(driver string, credentials string) (*sql.DB, error) {
			return nil, fmt.Errorf("db error")
		}
		engine := SqliteEngine{}
		if _, err := engine.Start(Database{}); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("Start", func(t *testing.T) {
		openDB = func(driver string, credentials string) (*sql.DB, error) {
			return nil, nil
		}
		engine, err := SqliteEngine{}.Start(Database{})
		if err != nil {
			t.Fatal(err)
		}
		eng := engine.(SqliteEngine)
		if eng.baseSQLEngine.driver != "sqlite3" {
			t.Errorf("expected sqlite3, got %s", eng.baseSQLEngine.driver)
		}
		if eng.baseSQLEngine.escapeChar != "\"" {
			t.Errorf("expected \", got %s", eng.baseSQLEngine.escapeChar)
		}
		if eng.baseSQLEngine.pHolderChar != "?" {
			t.Errorf("expected ?, got %s", eng.baseSQLEngine.pHolderChar)
		}
	})

	t.Run("DB", func(t *testing.T) {
		mockedDB.Reset()
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected conversion error (dbMocker to  *sql.DB)")
			}
		}()
		engine.DB()
	})

	t.Run("Tx", func(t *testing.T) {
		mockedDB.Reset()
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected conversion error (nil to  *sql.Tx)")
			}
		}()
		engine.Tx()
	})

	t.Run("BeginTxErr", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.err = fmt.Errorf("db error")
		if _, err := engine.BeginTx(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("BeginTx", func(t *testing.T) {
		mockedDB.Reset()
		if _, err := engine.BeginTx(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("TxSupport", func(t *testing.T) {
		mockedDB.Reset()
		if !engine.TxSupport() {
			t.Fatalf("expected true, got false")
		}
	})

	t.Run("StopErr", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.err = fmt.Errorf("db error")
		if err := engine.Stop(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("CommitTxErr", func(t *testing.T) {
		mockedDB.Reset()
		if err := engine.CommitTx(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("CommitTxDBErr", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.err = fmt.Errorf("db error")
		engine.baseSQLEngine.tx = mockedDB
		if err := engine.CommitTx(); err == nil {
			t.Error("expected error, got nil")
		}
		engine.baseSQLEngine.tx = nil
	})

	t.Run("RollbackTxErr", func(t *testing.T) {
		mockedDB.Reset()
		if err := engine.RollbackTx(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("RollbackTxDBErr", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.err = fmt.Errorf("db error")
		engine.baseSQLEngine.tx = mockedDB
		if err := engine.RollbackTx(); err == nil {
			t.Error("expected error, got nil")
		}
		engine.baseSQLEngine.tx = nil
	})

	t.Run("CreateTable", func(t *testing.T) {
		mockedDB.Reset()
		if err := engine.CreateTable(model, false); err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		stmt := mockedDB.queries[0].Stmt
		if !strings.HasPrefix(stmt, `CREATE TABLE IF NOT EXISTS "users_user"`) {
			t.Errorf(
				"expected query start: %s",
				`CREATE TABLE IF NOT EXISTS "users_user"`,
			)
		}
		if !strings.Contains(stmt, `"email" VARCHAR(100) NOT NULL`) {
			t.Errorf(
				"expected query to contain: %s",
				`"email" VARCHAR(100) NOT NULL`,
			)
		}
		if !strings.Contains(stmt, `"id" INTEGER`) {
			t.Errorf("expected query to contain: %s", `"id" INTEGER`)
		}
	})

	t.Run("RenameTable", func(t *testing.T) {
		mockedDB.Reset()
		newModel := &Model{meta: Options{Table: "new_table"}}
		if err := engine.RenameTable(model, newModel); err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `ALTER TABLE "users_user" RENAME TO "new_table"`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Errorf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
	})

	t.Run("DropTable", func(t *testing.T) {
		mockedDB.Reset()
		if err := engine.DropTable(model); err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `DROP TABLE "users_user"`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Errorf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
	})

	t.Run("AddIndex", func(t *testing.T) {
		mockedDB.Reset()
		if err := engine.AddIndex(model, "test_index", "email"); err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `CREATE INDEX "test_index" ON "users_user" ("email")`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Errorf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
	})

	t.Run("DropIndex", func(t *testing.T) {
		mockedDB.Reset()
		if err := engine.DropIndex(model, "test_index"); err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `DROP INDEX "test_index"`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Errorf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
	})

	t.Run("AddNotNullColumnNoDefault", func(t *testing.T) {
		mockedDB.Reset()
		fields := Fields{
			"active": BooleanField{},
		}
		if err := engine.AddColumns(model, fields); err == nil {
			t.Fatal("expected not null column no default error")
		}
	})

	t.Run("AddColumns", func(t *testing.T) {
		mockedDB.Reset()
		fields := Fields{
			"active":  BooleanField{DefaultFalse: true},
			"updated": DateTimeField{AutoNow: true, Null: true},
		}
		if err := engine.AddColumns(model, fields); err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 8 {
			t.Fatalf("expected 8 queries, got %d", len(mockedDB.queries))
		}
		stmt := mockedDB.queries[0].Stmt
		if !strings.HasPrefix(stmt, `ALTER TABLE "users_user" ADD COLUMN`) {
			t.Errorf(
				"expected query start: %s",
				`ALTER TABLE "users_user" ADD COLUMN`,
			)
		}
		expected := `UPDATE "users_user" SET "active" = ?`
		stmt = mockedDB.queries[2].Stmt
		if stmt != expected {
			t.Errorf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
		stmt = mockedDB.queries[3].Stmt
		if !strings.HasPrefix(stmt, `CREATE TABLE "users_user__new"`) {
			t.Fatalf(
				"expected query start: %s",
				`CREATE TABLE "users_user__new"`,
			)
		}
		stmt = mockedDB.queries[4].Stmt
		if !strings.HasPrefix(stmt, `INSERT INTO "users_user__new" SELECT`) {
			t.Fatalf(
				"expected query start: %s",
				`INSERT INTO "users_user__new" SELECT"`,
			)
		}
		stmt = mockedDB.queries[5].Stmt
		if stmt != `DROP TABLE "users_user"` {
			t.Fatalf(
				"expected:\n\n%s\n\ngot:\n\n%s",
				`DROP TABLE "users_user"`,
				stmt,
			)
		}
		stmt = mockedDB.queries[6].Stmt
		expected = `ALTER TABLE "users_user__new" RENAME TO "users_user"`
		if stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
		stmt = mockedDB.queries[7].Stmt
		expected = `CREATE INDEX "test_index" ON "users_user" ("email")`
		if stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
	})

	t.Run("AddColumnsDBError", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.err = fmt.Errorf("db error")
		fields := Fields{
			"is_superuser": BooleanField{},
			"created":      DateTimeField{AutoNowAdd: true},
		}
		if err := engine.AddColumns(model, fields); err == nil {
			t.Error("expected db error")
		}
	})

	t.Run("DropColumns", func(t *testing.T) {
		mockedDB.Reset()
		if err := engine.DropColumns(model, "active", "updated"); err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 5 {
			t.Fatalf("expected 5 queries, got %d", len(mockedDB.queries))
		}
		st := mockedDB.queries[0].Stmt
		if !strings.HasPrefix(st, `CREATE TABLE "users_user__new"`) {
			t.Fatalf(
				"expected query start: %s",
				`CREATE TABLE "users_user__new"`,
			)
		}
		st = mockedDB.queries[1].Stmt
		if !strings.HasPrefix(st, `INSERT INTO "users_user__new" SELECT`) {
			t.Fatalf(
				"expected query start: %s",
				`INSERT INTO "users_user__new" SELECT"`,
			)
		}
		st = mockedDB.queries[2].Stmt
		if st != `DROP TABLE "users_user"` {
			t.Fatalf(
				"expected:\n\n%s\n\ngot:\n\n%s", `DROP TABLE "users_user"`, st,
			)
		}
		st = mockedDB.queries[3].Stmt
		expected := `ALTER TABLE "users_user__new" RENAME TO "users_user"`
		if st != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, st)
		}
		st = mockedDB.queries[4].Stmt
		expected = `CREATE INDEX "test_index" ON "users_user" ("email")`
		if st != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, st)
		}
	})

	t.Run("DropColumnsDBError", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.err = fmt.Errorf("db error")
		if err := engine.DropColumns(model, "active", "updated"); err == nil {
			t.Error("expected db error")
		}

	})

	t.Run("SelectQuery", func(t *testing.T) {
		mockedDB.Reset()
		cond := Q{"active": true}.OrNot(
			Q{"email": "user@test.com"}.Or(Q{"pk >=": 10}),
		).AndNot(
			Q{"updated <": "2018-07-20"},
		)
		options := QueryOptions{
			Conditioner: cond,
			Fields:      []string{"id", "email"},
		}
		query, err := engine.SelectQuery(model, options)
		if err != nil {
			t.Fatal(err)
		}
		expected := `SELECT "id", "email" FROM "users_user" WHERE (` +
			`("active" = ?) OR NOT (("email" = ?) OR ("id" >= ?))` +
			`) AND NOT ("updated" < ?)`
		if query.Stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, query.Stmt)
		}
		if len(query.Args) != 4 {
			t.Fatalf("expected 4 query args, got %d", len(query.Args))
		}
		if val, ok := query.Args[0].(bool); !ok || !val {
			t.Errorf("expected true, got %s", query.Args[0])
		}
		if val, ok := query.Args[3].(string); !ok || val != "2018-07-20" {
			t.Errorf("expected true, got %s", query.Args[3])
		}
	})

	t.Run("SelectExclude", func(t *testing.T) {
		mockedDB.Reset()
		cond := Q{}.AndNot(Q{"email": "user@test.com"})
		options := QueryOptions{
			Conditioner: cond,
			Fields:      []string{"id"},
		}
		query, err := engine.SelectQuery(model, options)
		if err != nil {
			t.Fatal(err)
		}
		expected := `SELECT "id" FROM "users_user" WHERE NOT ("email" = ?)`
		if query.Stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, query.Stmt)
		}
	})

	t.Run("GetRows", func(t *testing.T) {
		mockedDB.Reset()
		options := QueryOptions{
			Conditioner: Q{"active": true},
			Fields:      []string{"id", "updated"},
			Start:       10,
			End:         20,
		}
		_, err := engine.GetRows(model, options)
		if err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `SELECT "id", "updated" FROM "users_user" ` +
			`WHERE "active" = ? LIMIT 10 OFFSET 10`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
	})

	t.Run("GetRowsNoLimit", func(t *testing.T) {
		mockedDB.Reset()
		options := QueryOptions{
			Conditioner: Q{"active": true},
			Fields:      []string{"id", "updated"},
			Start:       10,
			End:         -1,
		}
		_, err := engine.GetRows(model, options)
		if err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `SELECT "id", "updated" FROM "users_user" ` +
			`WHERE "active" = ? LIMIT -1 OFFSET 10`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
	})

	t.Run("GetRowsInvalidCondition", func(t *testing.T) {
		mockedDB.Reset()
		options := QueryOptions{
			Conditioner: Q{"username": "test"},
			Fields:      []string{"id", "updated"},
			Start:       10,
			End:         20,
		}
		_, err := engine.GetRows(model, options)
		if err == nil {
			t.Fatal("expected unknown field error")
		}
	})

	t.Run("InsertRow", func(t *testing.T) {
		mockedDB.Reset()
		values := Values{"email": "user@test.com", "active": true}
		_, err := engine.InsertRow(model, values)
		if err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		stmt := mockedDB.queries[0].Stmt
		if !strings.HasPrefix(stmt, `INSERT INTO "users_user"`) {
			t.Errorf("expected query start: %s", `INSERT INTO "users_user"`)
		}
		args := mockedDB.queries[0].Args
		if len(args) != 2 {
			t.Fatalf("expected 2 query args, got %d", len(args))
		}
	})

	t.Run("InsertRowDBError", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.err = fmt.Errorf("db error")
		values := Values{"email": "user@test.com", "active": true}
		_, err := engine.InsertRow(model, values)
		if err == nil {
			t.Fatal("expected db error")
		}
	})

	t.Run("InsertRowResultError", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.resultErr = fmt.Errorf("result error")
		values := Values{"email": "user@test.com", "active": true}
		_, err := engine.InsertRow(model, values)
		if err == nil {
			t.Fatal("expected result error")
		}
	})

	t.Run("UpdateRows", func(t *testing.T) {
		mockedDB.Reset()
		values := Values{"active": false}
		options := QueryOptions{Conditioner: Q{"active": true}}
		_, err := engine.UpdateRows(model, values, options)
		if err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `UPDATE "users_user" SET "active" = ? WHERE "active" = ?`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
		args := mockedDB.queries[0].Args
		if len(args) != 2 {
			t.Fatalf("expected one query args, got %d", len(args))
		}
		if val, ok := args[0].(bool); !ok || val {
			t.Errorf("expected false, got %s", args[0])
		}
	})

	t.Run("UpdateRowsResultError", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.resultErr = fmt.Errorf("result error")
		values := Values{"active": false}
		options := QueryOptions{Conditioner: Q{"active": true}}
		_, err := engine.UpdateRows(model, values, options)
		if err == nil {
			t.Fatal("expected result error")
		}
	})

	t.Run("DeleteRows", func(t *testing.T) {
		mockedDB.Reset()
		options := QueryOptions{Conditioner: Q{"id >=": 100}}
		_, err := engine.DeleteRows(model, options)
		if err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `DELETE FROM "users_user" WHERE "id" >= ?`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
		args := mockedDB.queries[0].Args
		if len(args) != 1 {
			t.Fatalf("expected one query args, got %d", len(args))
		}
		if val, ok := args[0].(int); !ok || val != 100 {
			t.Errorf("expected 100, got %s", args[0])
		}
	})

	t.Run("DeleteRowsResultError", func(t *testing.T) {
		mockedDB.Reset()
		mockedDB.resultErr = fmt.Errorf("result error")
		options := QueryOptions{Conditioner: Q{"id >=": 100}}
		_, err := engine.DeleteRows(model, options)
		if err == nil {
			t.Fatal("expected result error")
		}
	})

	t.Run("CountRows", func(t *testing.T) {
		mockedDB.Reset()
		options := QueryOptions{Conditioner: Q{"email": "user@test.com"}}
		_, err := engine.CountRows(model, options)
		if err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `SELECT COUNT(*) FROM "users_user" WHERE "email" = ?`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
		args := mockedDB.queries[0].Args
		if len(args) != 1 {
			t.Fatalf("expected one query args, got %d", len(args))
		}
		if val, ok := args[0].(string); !ok || val != "user@test.com" {
			t.Errorf("expected user@test.com, got %s", args[0])
		}
	})

	t.Run("CountInvalidCondition", func(t *testing.T) {
		mockedDB.Reset()
		options := QueryOptions{Conditioner: Q{"username": "user@test.com"}}
		_, err := engine.CountRows(model, options)
		if err == nil {
			t.Fatal("expected unknown field error")
		}
	})

	t.Run("CountDBError", func(t *testing.T) {
		mockedDB.Reset()
		origScanRow := scanRow
		defer func() { scanRow = origScanRow }()
		scanRow = func(ex sqlExecutor, dest interface{}, query Query) error {
			return fmt.Errorf("db error")
		}
		options := QueryOptions{Conditioner: Q{"email": "user@test.com"}}
		_, err := engine.CountRows(model, options)
		if err == nil {
			t.Fatal("expected db error")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		mockedDB.Reset()
		options := QueryOptions{Conditioner: Q{"email": "user@test.com"}}
		_, err := engine.Exists(model, options)
		if err != nil {
			t.Fatal(err)
		}
		if len(mockedDB.queries) != 1 {
			t.Fatalf("expected one query, got %d", len(mockedDB.queries))
		}
		expected := `SELECT EXISTS (` +
			`SELECT "id" FROM "users_user" WHERE "email" = ?)`
		stmt := mockedDB.queries[0].Stmt
		if stmt != expected {
			t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", expected, stmt)
		}
		args := mockedDB.queries[0].Args
		if len(args) != 1 {
			t.Fatalf("expected one query args, got %d", len(args))
		}
		if val, ok := args[0].(string); !ok || val != "user@test.com" {
			t.Errorf("expected user@test.com, got %s", args[0])
		}
	})

	t.Run("ExistsInvalidCondition", func(t *testing.T) {
		mockedDB.Reset()
		options := QueryOptions{Conditioner: Q{"username": "user@test.com"}}
		_, err := engine.Exists(model, options)
		if err == nil {
			t.Fatal("expected unknown field error")
		}
	})

	t.Run("ExistsDBError", func(t *testing.T) {
		mockedDB.Reset()
		origScanRow := scanRow
		defer func() { scanRow = origScanRow }()
		scanRow = func(ex sqlExecutor, dest interface{}, query Query) error {
			return fmt.Errorf("db error")
		}
		options := QueryOptions{Conditioner: Q{"email": "user@test.com"}}
		_, err := engine.Exists(model, options)
		if err == nil {
			t.Fatal("expected db error")
		}
	})
}
