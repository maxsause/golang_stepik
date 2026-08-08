package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type handler struct {
	DB     *sql.DB
	Tables []string
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	r := mux.NewRouter()
	tables, err := checkTables(db)
	if err != nil {
		return nil, err
	}

	h := &handler{
		DB:     db,
		Tables: tables,
	}

	r.HandleFunc("/", h.List).Methods(http.MethodGet)
	r.HandleFunc("/{table}", h.ListRecords).Methods(http.MethodGet)
	r.HandleFunc("/{table}/{id}", h.GetRecord).Methods(http.MethodGet)
	r.HandleFunc("/{table}/", h.Add).Methods(http.MethodPut)
	r.HandleFunc("/{table}/{id}", h.Update).Methods(http.MethodPost)
	r.HandleFunc("/{table}/{id}", h.Delete).Methods(http.MethodDelete)

	return r, nil
}

func (h *handler) List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"tables": h.Tables,
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *handler) ListRecords(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	if !containsTable(h.Tables, table) {
		w.WriteHeader(http.StatusNotFound)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unknown table",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	limit := 5
	offset := 0

	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			limit = parsed
		}
	}

	if value := r.URL.Query().Get("offset"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			offset = parsed
		}
	}

	query := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", table)
	rows, err := h.DB.Query(query, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var records []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		dest := make([]interface{}, len(columns))

		for i := range values {
			dest[i] = &values[i]
		}

		if err := rows.Scan(dest...); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		record := make(map[string]interface{})

		for i, value := range values {
			if b, ok := value.([]byte); ok {
				value = string(b)
			}
			record[columns[i]] = value
		}
		records = append(records, record)
	}

	if err = rows.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"records": records,
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (h *handler) GetRecord(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	if !containsTable(h.Tables, table) {
		w.WriteHeader(http.StatusNotFound)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unknown table",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	query := fmt.Sprintf("SELECT * FROM %s", table)
	rows, err := h.DB.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = rows.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idName := columns[0]
	id := vars["id"]

	query = fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", table, idName)
	rows, err = h.DB.Query(query, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !rows.Next() {
		w.WriteHeader(http.StatusNotFound)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "record not found",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	values := make([]interface{}, len(columns))
	dest := make([]interface{}, len(columns))

	for i := range values {
		dest[i] = &values[i]
	}

	if err := rows.Scan(dest...); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	answer := make(map[string]interface{})

	for i, value := range values {
		if b, ok := value.([]byte); ok {
			value = string(b)
		}
		answer[columns[i]] = value
	}

	if err = rows.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"record": answer,
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *handler) Add(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	if !containsTable(h.Tables, table) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unknown table",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	data := make(map[string]interface{})
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()

	if err := decoder.Decode(&data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "invalid json",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	query := fmt.Sprintf("SHOW FULL COLUMNS FROM %s", table)
	rows, err := h.DB.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metaColumns, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var (
		fields       []string
		placeholders []string
		values       []interface{}
		primaryKey   string
	)

	for rows.Next() {
		raw := make([]interface{}, len(metaColumns))
		dest := make([]interface{}, len(metaColumns))

		for i := range raw {
			dest[i] = &raw[i]
		}

		if err := rows.Scan(dest...); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		meta := make(map[string]string)

		for i, value := range raw {
			if value == nil {
				meta[metaColumns[i]] = ""
				continue
			}

			if b, ok := value.([]byte); ok {
				meta[metaColumns[i]] = string(b)
				continue
			}

			meta[metaColumns[i]] = fmt.Sprint(value)
		}

		field := meta["Field"]
		if meta["Key"] == "PRI" {
			primaryKey = field
		}
		if strings.Contains(meta["Extra"], "auto_increment") {
			continue
		}

		value, exists := data[field]
		if !exists {
			if meta["Null"] == "YES" {
				value = nil
			} else {
				sqlType := strings.ToLower(meta["Type"])

				if strings.Contains(sqlType, "int") {
					value = int64(0)
				} else if strings.Contains(sqlType, "float") ||
					strings.Contains(sqlType, "double") ||
					strings.Contains(sqlType, "decimal") {
					value = float64(0)
				} else {
					value = ""
				}
			}
		} else if value == nil {
			if meta["Null"] != "YES" {
				writeFieldError(w, field)
				return
			}
		} else {
			convertedValue, ok := convertValue(meta["Type"], value)
			if !ok {
				writeFieldError(w, field)
				return
			}
			value = convertedValue
		}

		fields = append(fields, "`"+field+"`")
		placeholders = append(placeholders, "?")
		values = append(values, value)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = rows.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	query = fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	result, err := h.DB.Exec(query, values...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			primaryKey: id,
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *handler) Update(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	id := vars["id"]

	if !containsTable(h.Tables, table) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unknown table",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	data := make(map[string]interface{})

	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()

	if err := decoder.Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := fmt.Sprintf("SHOW FULL COLUMNS FROM %s", table)

	rows, err := h.DB.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	metaColumns, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tableColumns := make(map[string]map[string]string)
	idName := ""

	for rows.Next() {
		values := make([]interface{}, len(metaColumns))
		dest := make([]interface{}, len(metaColumns))

		for i := range values {
			dest[i] = &values[i]
		}

		if err := rows.Scan(dest...); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		column := make(map[string]string)

		for i, value := range values {
			if value == nil {
				column[metaColumns[i]] = ""
				continue
			}

			if b, ok := value.([]byte); ok {
				column[metaColumns[i]] = string(b)
				continue
			}

			column[metaColumns[i]] = fmt.Sprint(value)
		}

		tableColumns[column["Field"]] = column

		if column["Key"] == "PRI" {
			idName = column["Field"]
		}
	}

	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = rows.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fields := make([]string, 0)
	values := make([]interface{}, 0)

	for field, value := range data {
		column, exists := tableColumns[field]

		if !exists {
			continue
		}
		if field == idName {
			writeFieldError(w, field)
			return
		}
		if value == nil {
			if column["Null"] != "YES" {
				writeFieldError(w, field)
				return
			}
		} else {
			columnType := strings.ToLower(column["Type"])

			if strings.Contains(columnType, "int") {
				number, ok := value.(json.Number)
				if !ok {
					writeFieldError(w, field)
					return
				}

				value, err = number.Int64()
				if err != nil {
					writeFieldError(w, field)
					return
				}
			} else {
				if _, ok := value.(string); !ok {
					writeFieldError(w, field)
					return
				}
			}
		}

		fields = append(fields, fmt.Sprintf("`%s` = ?", field))
		values = append(values, value)
	}

	if len(fields) == 0 {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"updated": 0,
			},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	values = append(values, id)

	query = fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = ?",
		table,
		strings.Join(fields, ", "),
		idName,
	)
	result, err := h.DB.Exec(query, values...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updated, err := result.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"updated": updated,
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *handler) Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	table := vars["table"]
	if !containsTable(h.Tables, table) {
		w.WriteHeader(http.StatusNotFound)
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "unknown table",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	query := fmt.Sprintf("SELECT * FROM %s", table)
	rows, err := h.DB.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = rows.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idName := columns[0]
	id := vars["id"]

	query = fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table, idName)

	result, err := h.DB.Exec(query, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"deleted": deleted,
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func checkTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	return tables, nil
}

func containsTable(tables []string, target string) bool {
	for _, table := range tables {
		if target == table {
			return true
		}
	}
	return false
}

func convertValue(sqlType string, value interface{}) (interface{}, bool) {
	sqlType = strings.ToLower(sqlType)

	number, isNumber := value.(json.Number)

	if strings.Contains(sqlType, "int") {
		if !isNumber {
			return nil, false
		}

		v, err := number.Int64()
		return v, err == nil
	}

	if strings.Contains(sqlType, "float") ||
		strings.Contains(sqlType, "double") ||
		strings.Contains(sqlType, "decimal") {

		if !isNumber {
			return nil, false
		}

		v, err := number.Float64()
		return v, err == nil
	}

	v, ok := value.(string)
	return v, ok
}

func writeFieldError(w http.ResponseWriter, field string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": fmt.Sprintf("field %s have invalid type", field),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
