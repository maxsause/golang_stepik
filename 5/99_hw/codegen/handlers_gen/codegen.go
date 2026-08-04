package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

const annotationPrefix = "// apigen:api "

type ApiMeta struct {
	URL    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

type MethodMeta struct {
	Receiver   string
	Name       string
	ParamsType string
	URL        string
	Auth       bool
	HTTPMethod string
}

type ParamsMeta struct {
	Name   string
	Fields []FieldMeta
}

type FieldMeta struct {
	Name      string
	Type      string
	ParamName string
	Required  bool
	Min       *int
	Max       *int
	Enum      []string
	Default   *string
}

func main() {
	if len(os.Args) != 3 {
		log.Fatalf(
			"usage: %s <input.go> <output.go>",
			os.Args[0],
		)
	}

	inputPath := os.Args[1]
	outputPath := os.Args[2]

	fset := token.NewFileSet()

	file, err := parser.ParseFile(
		fset,
		inputPath,
		nil,
		parser.ParseComments,
	)
	if err != nil {
		log.Fatal(err)
	}

	methods, err := parseMethods(file)
	if err != nil {
		log.Fatal(err)
	}

	paramsModels, err := parseParams(file, methods)
	if err != nil {
		log.Fatal(err)
	}

	groupedMethods := groupMethodsByReceiver(methods)

	var out bytes.Buffer

	if err := generateHeader(&out); err != nil {
		log.Fatal(err)
	}

	receivers := make([]string, 0, len(groupedMethods))

	for receiver := range groupedMethods {
		receivers = append(receivers, receiver)
	}

	sort.Strings(receivers)

	for _, receiver := range receivers {
		receiverMethods := groupedMethods[receiver]

		if err := generateServeHTTP(&out, receiver, receiverMethods); err != nil {
			log.Fatal(err)
		}

		if err := generateHandlers(&out, receiver, receiverMethods, paramsModels); err != nil {
			log.Fatal(err)
		}
	}

	formattedCode, err := format.Source(out.Bytes())
	if err != nil {
		fmt.Println("generated code:")
		fmt.Println(out.String())

		log.Fatalf("format generated code: %v", err)
	}

	if err := os.WriteFile(outputPath, formattedCode, 0o644); err != nil {
		log.Fatalf("write generated file %s: %v", outputPath, err)
	}
}

func parseMethods(file *ast.File) ([]MethodMeta, error) {
	var methods []MethodMeta

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if funcDecl.Doc == nil {
			continue
		}

		for _, comment := range funcDecl.Doc.List {
			if !strings.HasPrefix(comment.Text, annotationPrefix) {
				continue
			}
			annotation := strings.TrimPrefix(
				comment.Text,
				annotationPrefix,
			)
			var apiMeta ApiMeta
			if err := json.Unmarshal(
				[]byte(annotation),
				&apiMeta,
			); err != nil {
				return nil, fmt.Errorf(
					"parse annotation for %s: %w",
					funcDecl.Name.Name,
					err,
				)
			}
			receiverName, err := parseReceiverName(funcDecl)
			if err != nil {
				return nil, err
			}
			paramsType, err := parseParamsType(funcDecl)
			if err != nil {
				return nil, err
			}

			methods = append(methods, MethodMeta{
				Receiver:   receiverName,
				Name:       funcDecl.Name.Name,
				ParamsType: paramsType,
				URL:        apiMeta.URL,
				Auth:       apiMeta.Auth,
				HTTPMethod: apiMeta.Method,
			})
			break
		}
	}
	return methods, nil
}

func parseReceiverName(funcDecl *ast.FuncDecl) (string, error) {
	receiverField := funcDecl.Recv.List[0]
	starExpr, ok := receiverField.Type.(*ast.StarExpr)
	if !ok {
		return "", fmt.Errorf(
			"method %s has unsupported receiver",
			funcDecl.Name.Name,
		)
	}
	receiverIdent, ok := starExpr.X.(*ast.Ident)
	if !ok {
		return "", fmt.Errorf(
			"method %s has unsupported receiver type",
			funcDecl.Name.Name,
		)
	}
	return receiverIdent.Name, nil
}

func parseParamsType(funcDecl *ast.FuncDecl) (string, error) {
	params := funcDecl.Type.Params.List
	if len(params) < 2 {
		return "", fmt.Errorf(
			"method %s must have context and params arguments",
			funcDecl.Name.Name,
		)
	}
	paramsIdent, ok := params[1].Type.(*ast.Ident)
	if !ok {
		return "", fmt.Errorf(
			"method %s has unsupported params type",
			funcDecl.Name.Name,
		)
	}
	return paramsIdent.Name, nil
}

func parseParams(file *ast.File, methods []MethodMeta) (map[string]ParamsMeta, error) {
	requiredParams := make(map[string]struct{})
	for _, method := range methods {
		requiredParams[method.ParamsType] = struct{}{}
	}

	paramsModels := make(map[string]ParamsMeta)

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			typeName := typeSpec.Name.Name
			if _, required := requiredParams[typeName]; !required {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				return nil, fmt.Errorf(
					"%s must be a struct",
					typeName,
				)
			}
			structFields := make(
				[]FieldMeta,
				0,
				len(structType.Fields.List),
			)

			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					return nil, fmt.Errorf(
						"%s contains an embedded field",
						typeName,
					)
				}
				fieldType, ok := field.Type.(*ast.Ident)
				if !ok {
					return nil, fmt.Errorf(
						"field %s.%s has unsupported type",
						typeName,
						field.Names[0].Name,
					)
				}

				for _, fieldName := range field.Names {
					fieldMeta, err := parseField(
						fieldName.Name,
						fieldType.Name,
						field.Tag,
					)
					if err != nil {
						return nil, fmt.Errorf(
							"parse field %s.%s: %w",
							typeName,
							fieldName.Name,
							err,
						)
					}
					structFields = append(
						structFields,
						fieldMeta,
					)
				}
			}
			paramsModels[typeName] = ParamsMeta{
				Name:   typeName,
				Fields: structFields,
			}
		}
	}

	for requiredType := range requiredParams {
		if _, found := paramsModels[requiredType]; !found {
			return nil, fmt.Errorf(
				"params type %s not found",
				requiredType,
			)
		}
	}

	return paramsModels, nil
}

func parseField(name string, fieldType string, tagLiteral *ast.BasicLit) (FieldMeta, error) {
	fieldMeta := FieldMeta{
		Name:      name,
		Type:      fieldType,
		ParamName: strings.ToLower(name),
	}

	if fieldType != "string" && fieldType != "int" {
		return FieldMeta{}, fmt.Errorf(
			"unsupported field type %s",
			fieldType,
		)
	}
	if tagLiteral == nil {
		return fieldMeta, nil
	}
	fullTag, err := strconv.Unquote(tagLiteral.Value)
	if err != nil {
		return FieldMeta{}, fmt.Errorf(
			"unquote struct tag: %w",
			err,
		)
	}
	validatorTag := reflect.StructTag(fullTag).
		Get("apivalidator")
	if validatorTag == "" {
		return fieldMeta, nil
	}
	parts := strings.Split(validatorTag, ",")

	for _, part := range parts {
		if part == "required" {
			fieldMeta.Required = true
			continue
		}

		key, value, found := strings.Cut(part, "=")
		if !found {
			return FieldMeta{}, fmt.Errorf(
				"invalid validator rule %q",
				part,
			)
		}

		switch key {
		case "paramname":
			if value != "" {
				fieldMeta.ParamName = value
			}
		case "min":
			minValue, err := strconv.Atoi(value)
			if err != nil {
				return FieldMeta{}, fmt.Errorf(
					"invalid min value %q: %w",
					value,
					err,
				)
			}
			fieldMeta.Min = &minValue
		case "max":
			maxValue, err := strconv.Atoi(value)
			if err != nil {
				return FieldMeta{}, fmt.Errorf(
					"invalid max value %q: %w",
					value,
					err,
				)
			}
			fieldMeta.Max = &maxValue
		case "enum":
			fieldMeta.Enum = strings.Split(value, "|")
		case "default":
			defaultValue := value
			fieldMeta.Default = &defaultValue
		default:
			return FieldMeta{}, fmt.Errorf(
				"unknown validator rule %q",
				key,
			)
		}
	}
	return fieldMeta, nil
}

func intPointerValue(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func stringPointerValue(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func groupMethodsByReceiver(methods []MethodMeta) map[string][]MethodMeta {
	group := make(map[string][]MethodMeta)
	for _, method := range methods {
		group[method.Receiver] = append(group[method.Receiver], method)
	}
	return group
}

func generateHeader(out *bytes.Buffer) error {
	fmt.Fprintln(out, "// Code generated by handlers_gen; DO NOT EDIT.")
	fmt.Fprintln(out, "package main")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "import (")
	fmt.Fprintln(out, "\t\"encoding/json\"")
	fmt.Fprintln(out, "\t\"errors\"")
	fmt.Fprintln(out, "\t\"net/http\"")
	fmt.Fprintln(out, "\t\"strconv\"")
	fmt.Fprintln(out, ")")
	fmt.Fprintln(out)

	return nil
}

func generateServeHTTP(out *bytes.Buffer, receiver string, methods []MethodMeta) error {
	fmt.Fprintf(out, "func (srv *%s) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n", receiver)
	fmt.Fprintln(out, "\tswitch r.URL.Path {")

	for _, method := range methods {
		fmt.Fprintf(out, "\tcase %q:\n", method.URL)
		fmt.Fprintf(out, "\t\tsrv.handler%s(w, r)\n", method.Name)
	}

	fmt.Fprintln(out, "\tdefault:")
	fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
	fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusNotFound)")
	fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
	fmt.Fprintln(out, "\t\t\t\"error\": \"unknown method\",")
	fmt.Fprintln(out, "\t\t})")
	fmt.Fprintln(out, "\t}")
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	return nil
}

func generateHandlers(out *bytes.Buffer, receiver string, methods []MethodMeta, paramModels map[string]ParamsMeta) error {
	for _, method := range methods {
		params, found := paramModels[method.ParamsType]
		if !found {
			return fmt.Errorf("params model %s not found", method.ParamsType)
		}
		if err := generateHandler(out, receiver, method, params); err != nil {
			return err
		}
	}
	return nil
}

func generateHandler(out *bytes.Buffer, receiver string, method MethodMeta, params ParamsMeta) error {
	fmt.Fprintf(out, "func (srv *%s) handler%s(w http.ResponseWriter, r *http.Request) {\n", receiver, method.Name)

	if method.HTTPMethod != "" {
		fmt.Fprintf(out, "\tif r.Method != %q {\n", method.HTTPMethod)
		fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
		fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusNotAcceptable)")
		fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
		fmt.Fprintln(out, "\t\t\t\"error\": \"bad method\",")
		fmt.Fprintln(out, "\t\t})")
		fmt.Fprintln(out, "\t\treturn")
		fmt.Fprintln(out, "\t}")
		fmt.Fprintln(out)
	}

	if method.Auth {
		fmt.Fprintln(out, "\tif r.Header.Get(\"X-Auth\") != \"100500\" {")
		fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
		fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusForbidden)")
		fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
		fmt.Fprintln(out, "\t\t\t\"error\": \"unauthorized\",")
		fmt.Fprintln(out, "\t\t})")
		fmt.Fprintln(out, "\t\treturn")
		fmt.Fprintln(out, "\t}")
		fmt.Fprintln(out)
	}

	for _, field := range params.Fields {
		variableName := strings.ToLower(field.Name)

		fmt.Fprintf(out, "\t%sRaw := r.FormValue(%q)\n", variableName, field.ParamName)

		if field.Default != nil {
			fmt.Fprintf(out, "\tif %sRaw == \"\" {\n", variableName)
			fmt.Fprintf(out, "\t\t%sRaw = %q\n", variableName, *field.Default)
			fmt.Fprintln(out, "\t}")
		}

		if field.Required {
			errorMessage := fmt.Sprintf("%s must me not empty", field.ParamName)

			fmt.Fprintf(out, "\tif %sRaw == \"\" {\n", variableName)
			fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
			fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
			fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
			fmt.Fprintf(out, "\t\t\t\"error\": %q,\n", errorMessage)
			fmt.Fprintln(out, "\t\t})")
			fmt.Fprintln(out, "\t\treturn")
			fmt.Fprintln(out, "\t}")
		}

		switch field.Type {
		case "string":
			if field.Min != nil {
				errorMessage := fmt.Sprintf("%s len must be >= %d", field.ParamName, *field.Min)

				fmt.Fprintf(out, "\tif len(%sRaw) < %d {\n", variableName, *field.Min)
				fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
				fmt.Fprintf(out, "\t\t\t\"error\": %q,\n", errorMessage)
				fmt.Fprintln(out, "\t\t})")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}

			if field.Max != nil {
				errorMessage := fmt.Sprintf("%s len must be <= %d", field.ParamName, *field.Max)

				fmt.Fprintf(out, "\tif len(%sRaw) > %d {\n", variableName, *field.Max)
				fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
				fmt.Fprintf(out, "\t\t\t\"error\": %q,\n", errorMessage)
				fmt.Fprintln(out, "\t\t})")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}

			if len(field.Enum) > 0 {
				fmt.Fprint(out, "\tif ")

				for index, allowedValue := range field.Enum {
					if index > 0 {
						fmt.Fprint(out, " && ")
					}

					fmt.Fprintf(out, "%sRaw != %q", variableName, allowedValue)
				}

				fmt.Fprintln(out, " {")

				errorMessage := fmt.Sprintf("%s must be one of [%s]", field.ParamName, strings.Join(field.Enum, ", "))

				fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
				fmt.Fprintf(out, "\t\t\t\"error\": %q,\n", errorMessage)
				fmt.Fprintln(out, "\t\t})")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}

		case "int":
			fmt.Fprintf(out, "\t%s, err := strconv.Atoi(%sRaw)\n", variableName, variableName)

			errorMessage := fmt.Sprintf("%s must be int", field.ParamName)

			fmt.Fprintln(out, "\tif err != nil {")
			fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
			fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
			fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
			fmt.Fprintf(out, "\t\t\t\"error\": %q,\n", errorMessage)
			fmt.Fprintln(out, "\t\t})")
			fmt.Fprintln(out, "\t\treturn")
			fmt.Fprintln(out, "\t}")

			if field.Min != nil {
				errorMessage := fmt.Sprintf("%s must be >= %d", field.ParamName, *field.Min)

				fmt.Fprintf(out, "\tif %s < %d {\n", variableName, *field.Min)
				fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
				fmt.Fprintf(out, "\t\t\t\"error\": %q,\n", errorMessage)
				fmt.Fprintln(out, "\t\t})")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}

			if field.Max != nil {
				errorMessage := fmt.Sprintf("%s must be <= %d", field.ParamName, *field.Max)

				fmt.Fprintf(out, "\tif %s > %d {\n", variableName, *field.Max)
				fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
				fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusBadRequest)")
				fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
				fmt.Fprintf(out, "\t\t\t\"error\": %q,\n", errorMessage)
				fmt.Fprintln(out, "\t\t})")
				fmt.Fprintln(out, "\t\treturn")
				fmt.Fprintln(out, "\t}")
			}

		default:
			return fmt.Errorf("unsupported field type %q in %s.%s", field.Type, params.Name, field.Name)
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintf(out, "\tparams := %s{\n", params.Name)

	for _, field := range params.Fields {
		variableName := strings.ToLower(field.Name)

		switch field.Type {
		case "string":
			fmt.Fprintf(out, "\t\t%s: %sRaw,\n", field.Name, variableName)
		case "int":
			fmt.Fprintf(out, "\t\t%s: %s,\n", field.Name, variableName)
		default:
			return fmt.Errorf("unsupported field type %q in %s.%s", field.Type, params.Name, field.Name)
		}
	}

	fmt.Fprintln(out, "\t}")
	fmt.Fprintln(out)

	fmt.Fprintf(out, "\tresult, err := srv.%s(r.Context(), params)\n", method.Name)

	fmt.Fprintln(out, "\tif err != nil {")
	fmt.Fprintln(out, "\t\tvar apiErr ApiError")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "\t\tif errors.As(err, &apiErr) {")
	fmt.Fprintln(out, "\t\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
	fmt.Fprintln(out, "\t\t\tw.WriteHeader(apiErr.HTTPStatus)")
	fmt.Fprintln(out, "\t\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
	fmt.Fprintln(out, "\t\t\t\t\"error\": apiErr.Err.Error(),")
	fmt.Fprintln(out, "\t\t\t})")
	fmt.Fprintln(out, "\t\t\treturn")
	fmt.Fprintln(out, "\t\t}")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "\t\tw.Header().Set(\"Content-Type\", \"application/json\")")
	fmt.Fprintln(out, "\t\tw.WriteHeader(http.StatusInternalServerError)")
	fmt.Fprintln(out, "\t\t_ = json.NewEncoder(w).Encode(map[string]string{")
	fmt.Fprintln(out, "\t\t\t\"error\": err.Error(),")
	fmt.Fprintln(out, "\t\t})")
	fmt.Fprintln(out, "\t\treturn")
	fmt.Fprintln(out, "\t}")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "\tw.Header().Set(\"Content-Type\", \"application/json\")")
	fmt.Fprintln(out, "\t_ = json.NewEncoder(w).Encode(map[string]interface{}{")
	fmt.Fprintln(out, "\t\t\"error\": \"\",")
	fmt.Fprintln(out, "\t\t\"response\": result,")
	fmt.Fprintln(out, "\t})")

	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	return nil
}
