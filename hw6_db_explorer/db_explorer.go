package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные
var (
	errUnknownTable = errors.New("unknown table")
	errRecNotFound  = errors.New("record not found")
	errIDNotAllowed = errors.New("field id have invalid type")
)

func NewDecodingError(text string) error {
	return &DecodingError{text}
}

type DecodingError struct {
	text string
}

func (e *DecodingError) Error() string {
	return e.text
}

type Response struct {
	Error    string      `json:"error,omitempty"`
	Response interface{} `json:"response,omitempty"`
}

func (r *Response) Serialize() ([]byte, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return data, nil
}

//Handler  ...
type server struct {
	Store  *Store
	Tables map[string][]*TableInfo
}

type Store struct {
	DB *sql.DB
}

func (s *server) error(w http.ResponseWriter, r *http.Request, code int, err error) {
	res := &Response{Error: err.Error()}
	s.respond(w, r, code, res)
}

func (s *server) respond(w http.ResponseWriter, r *http.Request, code int, data *Response) {
	w.WriteHeader(code)
	// w.Header().Set("Content-Type", "application/json")
	d, _ := data.Serialize()
	w.Write(d)

}

func (h *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		h.getTablesHandler(w, r)
	default:
		table := strings.Split(r.URL.Path, "/")[1]
		_, ok := h.Tables[table]
		if !ok {
			h.error(w, r, http.StatusNotFound, errUnknownTable)
			return
		}
		if len(strings.Split(r.URL.Path, "/")) == 2 {
			if r.Method == "GET" {
				h.getTableContentsHandler(w, r, table)
			}

		} else {
			if r.Method == "GET" {
				id, err := strconv.Atoi(strings.Split(r.URL.Path, "/")[2])
				if err != nil {
					h.error(w, r, http.StatusBadRequest, err)
					return
				}
				h.getTableContentByIdHandler(w, r, table, id)
			} else if r.Method == "PUT" {
				h.createTableContentByIdHandler(w, r, table)
			} else if r.Method == "POST" {
				id, err := strconv.Atoi(strings.Split(r.URL.Path, "/")[2])
				if err != nil {

					h.error(w, r, http.StatusBadRequest, err)
					return
				}
				h.updateTableContentByIdHandler(w, r, table, id)
			} else if r.Method == "DELETE" {
				id, err := strconv.Atoi(strings.Split(r.URL.Path, "/")[2])
				if err != nil {

					h.error(w, r, http.StatusBadRequest, err)
					return
				}
				h.deleteTableContentByIdHandler(w, r, table, id)
			}
		}
	}
}

func (h *server) deleteTableContentByIdHandler(w http.ResponseWriter, r *http.Request, table string, id int) {
	result, err := h.Store.deleteTableContentById(h.Tables[table], table, id)
	if err != nil {
		if err == sql.ErrNoRows {
			h.error(w, r, http.StatusNotFound, errRecNotFound)
			return
		}
		h.error(w, r, http.StatusInternalServerError, err)
		return
	}

	h.respond(w, r, http.StatusOK, &Response{
		Response: map[string]interface{}{
			"deleted": result,
		},
	})
}

func (s *Store) deleteTableContentById(tInfos []*TableInfo, table string, id int) (int64, error) {
	var idColumn string
	for _, t := range tInfos {
		if t.Key == "PRI" {
			idColumn = t.Field
		}
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table, idColumn)
	res, err := s.DB.Exec(q, id)
	if err != nil {
		return 0, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (h *server) updateTableContentByIdHandler(w http.ResponseWriter, r *http.Request, table string, id int) {
	tInfos, _ := h.Tables[table]
	var body map[string]interface{}
	data, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err := json.Unmarshal(data, &body); err != nil {
		h.error(w, r, http.StatusInternalServerError, err)
		return
	}

	affectedRows, err := h.Store.updateTableContent(tInfos, table, body, id)
	if err != nil {
		switch err.(type) {
		case *DecodingError:
			h.error(w, r, http.StatusBadRequest, err)
			return
		}
		if err == errIDNotAllowed {
			h.error(w, r, http.StatusBadRequest, err)
			return
		}
		h.error(w, r, http.StatusInternalServerError, err)
		return
	}
	h.respond(w, r, http.StatusOK, &Response{
		Response: map[string]interface{}{
			"updated": affectedRows,
		},
	})
}

func (s *Store) updateTableContent(tInfos []*TableInfo, table string, data map[string]interface{}, id int) (int64, error) {
	values := make([]interface{}, 0)
	columns := make([]string, 0)
	var idColumn string
	for _, t := range tInfos {
		if t.Key == "PRI" {
			idColumn = t.Field
		}
	}

	for _, v := range tInfos {
		if _, ok := data[v.Field]; ok {
			if v.Field != idColumn {
				columns = append(columns, v.Field)
				if v.Type == "int" {
					val, ok := data[v.Field].(float64)
					if !ok {
						return 0, NewDecodingError(fmt.Sprintf("field %v have invalid type", v.Field))
					}
					values = append(values, int(val))
				} else {
					if data[v.Field] == nil {
						if v.Null == "NO" {
							return 0, NewDecodingError(fmt.Sprintf("field %v have invalid type", v.Field))
						}
						values = append(values, nil)
					} else {
						val, ok := data[v.Field].(string)
						if !ok {
							return 0, NewDecodingError(fmt.Sprintf("field %v have invalid type", v.Field))
						}
						values = append(values, val)
					}

				}
			} else {
				return 0, NewDecodingError(fmt.Sprintf("field %v have invalid type", v.Field))
			}

		}
	}
	sqlColumns := strings.Join(columns, "=?,")
	sqlColumns = sqlColumns + "=?"

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %d", table, sqlColumns, idColumn, id)

	res, err := s.DB.Exec(query, values...)
	if err != nil {
		return 0, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (h *server) createTableContentByIdHandler(w http.ResponseWriter, r *http.Request, table string) {
	tInfos, _ := h.Tables[table]
	var body map[string]interface{}
	data, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err := json.Unmarshal(data, &body); err != nil {
		h.error(w, r, http.StatusInternalServerError, err)
		return
	}

	idCol, id, err := h.Store.createTableContent(tInfos, table, body)
	if err != nil {
		switch err.(type) {
		case *DecodingError:
			h.error(w, r, http.StatusBadRequest, err)
			return
		}
		h.error(w, r, http.StatusInternalServerError, err)
		return
	}
	h.respond(w, r, http.StatusOK, &Response{
		Response: map[string]interface{}{
			idCol: id,
		},
	})
}

func (s *Store) createTableContent(tInfos []*TableInfo, table string, data map[string]interface{}) (string, int64, error) {
	values := make([]interface{}, 0)
	columns := make([]string, 0)
	var idColumn string
	for _, t := range tInfos {
		if t.Key == "PRI" {
			idColumn = t.Field
		}
	}
	for _, v := range tInfos {
		if _, ok := data[v.Field]; ok {
			if v.Field != idColumn {
				columns = append(columns, v.Field)
				if v.Type == "int" {
					val, ok := data[v.Field].(float64)
					if !ok {
						return idColumn, 0, NewDecodingError(fmt.Sprintf("field %v have invalid type", v.Field))
					}
					values = append(values, int(val))
				} else {
					if data[v.Field] == nil {
						if v.Null == "NO" {
							return idColumn, 0, NewDecodingError(fmt.Sprintf("field %v have invalid type", v.Field))
						}
						values = append(values, nil)
					} else {
						val, ok := data[v.Field].(string)
						if !ok {
							return idColumn, 0, NewDecodingError(fmt.Sprintf("field %v have invalid type", v.Field))
						}
						values = append(values, val)
					}
				}
			}
		} else if v.Null == "NO" {
			columns = append(columns, v.Field)
			if v.Type == "int" {
				values = append(values, 1)
			} else {
				values = append(values, "")
			}
		}
	}
	sqlColumns := strings.Join(columns, ",")

	sqlParams := strings.Repeat("?,", len(columns))
	sqlParams = sqlParams[:len(sqlParams)-1]

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES(%s)", table, sqlColumns, sqlParams)
	res, err := s.DB.Exec(query, values...)
	if err != nil {
		return idColumn, 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return idColumn, 0, err
	}
	return idColumn, id, nil
}

func (s *Store) getTableContents(tInfos []*TableInfo, table string, limit, offset int) ([]map[string]interface{}, error) {
	q := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", table)
	rows, err := s.DB.Query(q, limit, offset)
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	columns, _ := rows.Columns()
	result := make([]map[string]interface{}, 0)

	for rows.Next() {
		row := make(map[string]interface{})
		values := make([]interface{}, len(tInfos))
		for i := range values {
			if tInfos[i].Type == "int" {
				values[i] = new(int64)
			} else {
				values[i] = new(*string)
			}
		}
		err := rows.Scan(values...)
		if err != nil {
			return nil, err
		}
		for i, column := range columns {
			row[column] = values[i]
		}
		result = append(result, row)
	}
	// MyPrint(result)
	return result, nil
}

func myCustomCast(value interface{}) interface{} {
	switch value.(type) {
	case int:
		res, _ := value.(int)
		return res
	case string:
		res, _ := value.(string)
		return res
	case float32:
		res, _ := value.(float32)
		return res
	}
	return nil
}

func (h *server) getTablesHandler(w http.ResponseWriter, r *http.Request) {
	tables := make([]string, 0)
	for table := range h.Tables {
		tables = append(tables, table)
	}
	res := &Response{
		Response: map[string][]string{
			"tables": tables,
		},
	}

	h.respond(w, r, http.StatusOK, res)
}

//TableInfo ...
type TableInfo struct {
	Field      string
	Type       string
	Collation  *string
	Null       string
	Key        string
	Default    *string
	Extra      string
	Privileges string
	Comment    string
}

//NewDbExplorer ...
func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	server := &server{Store: &Store{DB: db}, Tables: make(map[string][]*TableInfo)}
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableName string
	for rows.Next() {
		err = rows.Scan(&tableName)
		if err != nil {
			return nil, err
		}

		server.Tables[tableName] = make([]*TableInfo, 0)
	}

	for table := range server.Tables {
		tInfos, err := server.Store.loadTableInfo(table)
		if err != nil {
			return nil, err
		}
		server.Tables[table] = tInfos
	}

	return server, nil
}

func (h *Store) loadTableInfo(tableName string) ([]*TableInfo, error) {
	tInfos := make([]*TableInfo, 0)
	rows, err := h.DB.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM %s", tableName))
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tInfo *TableInfo = &TableInfo{}
		err = rows.Scan(&tInfo.Field, &tInfo.Type, &tInfo.Collation, &tInfo.Null, &tInfo.Key, &tInfo.Default, &tInfo.Extra, &tInfo.Privileges, &tInfo.Comment)
		if err != nil {
			return nil, err
		}
		tInfos = append(tInfos, tInfo)

	}

	return tInfos, nil
}

func (s *Store) getTableContent(tInfos []*TableInfo, table string, id int) (map[string]interface{}, error) {
	var idColumn string
	for _, t := range tInfos {
		if t.Key == "PRI" {
			idColumn = t.Field
		}
	}

	q := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", table, idColumn)
	values := make([]interface{}, len(tInfos))
	for i := range values {
		if tInfos[i].Type == "int" {
			values[i] = new(int32)
		} else {
			values[i] = new(*string)
		}
	}
	err := s.DB.QueryRow(q, id).Scan(values...)
	if err != nil {
		return nil, err
	}

	item := make(map[string]interface{})
	for i := range values {
		item[tInfos[i].Field] = values[i]
	}
	// MyPrint(item)
	return item, nil
}

func (h *server) getTableContentByIdHandler(w http.ResponseWriter, r *http.Request, table string, id int) {
	result, err := h.Store.getTableContent(h.Tables[table], table, id)
	if err != nil {
		if err == sql.ErrNoRows {
			h.error(w, r, http.StatusNotFound, errRecNotFound)
			return
		}
		h.error(w, r, http.StatusInternalServerError, err)
		return
	}

	h.respond(w, r, http.StatusOK, &Response{
		Response: map[string]map[string]interface{}{
			"record": result,
		},
	})
}
func (h *server) getTableContentsHandler(w http.ResponseWriter, r *http.Request, table string) {

	q := r.URL.Query()
	limit := q.Get("limit")
	if limit == "" {
		limit = "5"
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		limitInt = 5
	}

	offset := q.Get("offset")
	if offset == "" {
		offset = "0"
	}
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		offsetInt = 0
	}

	results, err := h.Store.getTableContents(h.Tables[table], table, limitInt, offsetInt)
	if err != nil {
		h.error(w, r, http.StatusInternalServerError, err)
		return
	}

	h.respond(w, r, http.StatusOK, &Response{
		Response: map[string]interface{}{
			"records": results,
		},
	})
}
