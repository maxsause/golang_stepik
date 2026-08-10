package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type handler struct {
	DB     *sql.DB
	Tables []string
}

type Column struct {
	Name          string
	Type          string
	Nullable      bool
	PrimaryKey    bool
	AutoIncrement bool
}

type ListTablesResponse struct {
	Response TablesResponse `json:"response"`
}

type TablesResponse struct {
	Tables []string `json:"tables"`
}

type ListRecordsResponse struct {
	Response RecordsResponse `json:"response"`
}

type RecordsResponse struct {
	Records []map[string]interface{} `json:"records"`
}

type GetRecordResponse struct {
	Response RecordResponse `json:"response"`
}

type RecordResponse struct {
	Record map[string]interface{} `json:"record"`
}

type AddResponse struct {
	Response map[string]int64 `json:"response"`
}

type UpdatedResponse struct {
	Response UpdateResponse `json:"response"`
}

type UpdateResponse struct {
	Update int64 `json:"updated"`
}

type DeletedResponse struct {
	Response DeleteResponse `json:"response"`
}

type DeleteResponse struct {
	Delete int64 `json:"deleted"`
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	r := http.NewServeMux()

	tables, err := checkTables(db)
	if err != nil {
		return nil, err
	}

	h := &handler{
		DB:     db,
		Tables: tables,
	}

	r.HandleFunc("GET /", h.List)
	r.HandleFunc("GET /{table}", h.checkTable(h.ListRecords))
	r.HandleFunc("GET /{table}/{id}", h.checkTable(h.GetRecord))
	r.HandleFunc("PUT /{table}/", h.checkTable(h.Add))
	r.HandleFunc("POST /{table}/{id}", h.checkTable(h.Update))
	r.HandleFunc("DELETE /{table}/{id}", h.checkTable(h.Delete))

	return r, nil
}

func (h *handler) List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := ListTablesResponse{
		Response: TablesResponse{
			Tables: h.Tables,
		},
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		err = fmt.Errorf("list: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *handler) ListRecords(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
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
		err = fmt.Errorf("list records: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		err = fmt.Errorf("list records: %w", err)
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

		if err = rows.Scan(dest...); err != nil {
			err = fmt.Errorf("list records: %w", err)
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

	w.Header().Set("Content-Type", "application/json")
	resp := ListRecordsResponse{
		Response: RecordsResponse{
			Records: records,
		},
	}
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		err = fmt.Errorf("list records: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (h *handler) GetRecord(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	query := fmt.Sprintf("SELECT * FROM %s", table)
	rows, err := h.DB.Query(query)
	if err != nil {
		err = fmt.Errorf("get record: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		err = fmt.Errorf("get record: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = rows.Close(); err != nil {
		err = fmt.Errorf("get record: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idName := columns[0]
	id := r.PathValue("id")

	query = fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", table, idName)
	rows, err = h.DB.Query(query, id)
	if err != nil {
		err = fmt.Errorf("get record: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		httpError(w, "record not found", http.StatusNotFound)
		return
	}

	values := make([]interface{}, len(columns))
	dest := make([]interface{}, len(columns))

	for i := range values {
		dest[i] = &values[i]
	}

	if err = rows.Scan(dest...); err != nil {
		err = fmt.Errorf("get record: %w", err)
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

	w.Header().Set("Content-Type", "application/json")
	resp := GetRecordResponse{
		Response: RecordResponse{
			Record: answer,
		},
	}
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		err = fmt.Errorf("get record: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *handler) Add(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	data := make(map[string]interface{})
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()

	if err := decoder.Decode(&data); err != nil {
		httpError(w, "invalid json", http.StatusBadRequest)
		return
	}

	columns, err := getTableColumns(h.DB, table)
	if err != nil {
		err = fmt.Errorf("add: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var (
		fields       []string
		placeholders []string
		values       []interface{}
		primaryKey   string
	)

	for _, column := range columns {
		if column.PrimaryKey {
			primaryKey = column.Name
		}

		if column.AutoIncrement {
			continue
		}

		value, exists := data[column.Name]

		if !exists {
			value = defaultValue(column)
		} else if value == nil {
			if !column.Nullable {
				writeFieldError(w, column.Name)
				return
			}
		} else {
			convertedValue, ok := convertValue(column.Type, value)
			if !ok {
				writeFieldError(w, column.Name)
				return
			}
			value = convertedValue
		}

		fields = append(fields, "`"+column.Name+"`")
		placeholders = append(placeholders, "?")
		values = append(values, value)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	result, err := h.DB.Exec(query, values...)
	if err != nil {
		err = fmt.Errorf("add: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		err = fmt.Errorf("add: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := AddResponse{
		Response: map[string]int64{
			primaryKey: id,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		err = fmt.Errorf("add: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *handler) Update(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	id := r.PathValue("id")

	data := make(map[string]interface{})

	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()

	if err := decoder.Decode(&data); err != nil {
		httpError(w, "invalid json", http.StatusBadRequest)
		return
	}

	columns, err := getTableColumns(h.DB, table)
	if err != nil {
		err = fmt.Errorf("update: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tableColumns := columnsByName(columns)
	idName := primaryKey(columns)

	fields := make([]string, 0)
	values := make([]interface{}, 0)

	for field, value := range data {
		column, exists := tableColumns[field]

		if !exists {
			continue
		}
		if column.PrimaryKey {
			writeFieldError(w, field)
			return
		}
		if value == nil {
			if !column.Nullable {
				writeFieldError(w, field)
				return
			}
		} else {
			convertedValue, ok := convertValue(column.Type, value)
			if !ok {
				writeFieldError(w, field)
				return
			}

			value = convertedValue
		}

		fields = append(fields, fmt.Sprintf("`%s` = ?", field))
		values = append(values, value)
	}

	values = append(values, id)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = ?",
		table,
		strings.Join(fields, ", "),
		idName,
	)

	result, err := h.DB.Exec(query, values...)
	if err != nil {
		err = fmt.Errorf("update: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	updated, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("update: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := UpdatedResponse{
		Response: UpdateResponse{
			Update: updated,
		},
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(resp); err != nil {
		err = fmt.Errorf("update: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *handler) Delete(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")

	query := fmt.Sprintf("SELECT * FROM %s", table)
	rows, err := h.DB.Query(query)
	if err != nil {
		err = fmt.Errorf("delete: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		err = fmt.Errorf("delete: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = rows.Close(); err != nil {
		err = fmt.Errorf("delete: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	idName := columns[0]
	id := r.PathValue("id")

	query = fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table, idName)

	result, err := h.DB.Exec(query, id)
	if err != nil {
		err = fmt.Errorf("delete: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("delete: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := DeletedResponse{
		Response: DeleteResponse{
			Delete: deleted,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		err = fmt.Errorf("delete: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func checkTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
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
		err = fmt.Errorf("write field error: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func httpError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *handler) checkTable(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := r.PathValue("table")
		if !containsTable(h.Tables, table) {
			httpError(w, "unknown table", http.StatusNotFound)
			return
		}
		next(w, r)
	}
}

func getTableColumns(db *sql.DB, table string) ([]Column, error) {
	query := fmt.Sprintf("SHOW FULL COLUMNS FROM %s", table)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []Column

	for rows.Next() {
		var (
			field      string
			columnType string
			collation  sql.NullString
			nullable   string
			key        string
			defaultVal interface{}
			extra      string
			privileges string
			comment    string
		)

		if err = rows.Scan(
			&field,
			&columnType,
			&collation,
			&nullable,
			&key,
			&defaultVal,
			&extra,
			&privileges,
			&comment,
		); err != nil {
			return nil, err
		}

		columns = append(columns, Column{
			Name:          field,
			Type:          columnType,
			Nullable:      nullable == "YES",
			PrimaryKey:    key == "PRI",
			AutoIncrement: strings.Contains(extra, "auto_increment"),
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return columns, nil
}

func defaultValue(column Column) interface{} {
	if column.Nullable {
		return nil
	}

	sqlType := strings.ToLower(column.Type)

	switch {
	case strings.Contains(sqlType, "int"):
		return int64(0)

	case strings.Contains(sqlType, "float"),
		strings.Contains(sqlType, "double"),
		strings.Contains(sqlType, "decimal"):
		return float64(0)

	default:
		return ""
	}
}

func columnsByName(columns []Column) map[string]Column {
	result := make(map[string]Column, len(columns))

	for _, column := range columns {
		result[column.Name] = column
	}
	return result
}

func primaryKey(columns []Column) string {
	for _, column := range columns {
		if column.PrimaryKey {
			return column.Name
		}
	}
	return ""
}
