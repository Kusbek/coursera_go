package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"
)

// код писать тут

var mainTmpl *template.Template
var handlerTmpl *template.Template
var serveHttpTmpl *template.Template

func init() {
	mainTmpl = template.Must(template.New("mainTmpl").Parse(`
package {{.}}
		
import (
	"net/http"
	"net/url"
	"fmt"
	"strconv"
	"strings"
	"context"
	"encoding/json"
)

type response struct {
	Error    string      ` + "`" + `json:"error"` + "`" + `
	Response interface{} ` + "`" + `json:"response,omitempty"` + "`" + `
}

func (r *response) String() string {
	data, _ := json.Marshal(r)
	return string(data)
}
`,
	))

	handlerTmpl = template.Must(template.New("handlerTmpl").Parse(`
func (srv *{{.RecName}}) {{.Name}}Handler(w http.ResponseWriter, r *http.Request){
	var param *{{.InputParams}} =  &{{.InputParams}}{}
	{{if (eq .Method "POST")}} 
	if r.Method != "POST" {
		err := response{Error:"bad method"}
		http.Error(w, err.String(), http.StatusNotAcceptable)
		return
	}
	{{if .Auth}}
	if r.Header.Get("X-Auth") != "100500" {
		body := response{Error: "unauthorized"}
		http.Error(w, body.String(), http.StatusForbidden)
		return
	}
	{{end}}
	r.ParseForm()
	urlVal := r.Form
	{{else}}
	if r.Method != "POST" && r.Method != "GET"{
		http.Error(w, "Bad method", http.StatusNotAcceptable)
		return
	}
	var urlVal url.Values
	if r.Method == "GET" {
		urlVal = r.URL.Query()
	}else{
		r.ParseForm()
		urlVal = r.Form
	}
	{{end}}

	err := param.Unmarshall(urlVal)
	if err != nil {
		res := response{Error:err.Error()}
		http.Error(w, res.String(), http.StatusBadRequest)
		return
	}

	res, err := srv.{{.Name}}(context.Background(), *param)
	if err != nil {
		body := response{Error: err.Error()}
		if err, ok := err.(ApiError); ok {
			http.Error(w, body.String(), err.HTTPStatus)
			return
		}
		http.Error(w, body.String(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	body := response{Error: "", Response: res}
	w.Write([]byte(body.String()))

}
		`,
	))

	serveHttpTmpl = template.Must(template.New("serveHttpTmpl").Parse(`
func (srv  *{{.Rcv}}) ServeHTTP (w http.ResponseWriter, r *http.Request){
	switch r.URL.Path {
	{{range .Infos -}}
	case "{{.ServerParams.Url}}": 
		srv.{{.Handler.Name}}Handler (w,r)
	{{end -}}
	default: 
		err := response{Error:"unknown method"}
  		http.Error(w, err.String(), http.StatusNotFound)
	}	
}
		`,
	))

}

type ServerInfo struct {
	ServerParams *Params
	Handler      *Handler
}

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	out, _ := os.Create(os.Args[2])
	mainTmpl.Execute(out, node.Name.Name)
	servers := make(map[string][]*ServerInfo)
	for _, v := range node.Decls {
		switch decl := v.(type) {
		case *ast.FuncDecl:
			if decl.Doc != nil && strings.HasPrefix(decl.Doc.Text(), "apigen:api") {
				httpParams := generateHttpParamsForServer(decl.Doc.Text())
				srvName := getReceiverName(decl)
				handler := generateHandler(out, decl, httpParams.Auth, httpParams.Method, srvName)
				info := &ServerInfo{
					ServerParams: httpParams,
					Handler:      handler,
				}
				servers[srvName] = append(servers[srvName], info)
			}

		}
	}
	ast.Inspect(node, func(n ast.Node) bool {

		switch v := n.(type) {

		case *ast.TypeSpec:
			if strct, ok := v.Type.(*ast.StructType); ok {
				if isParamStruct(strct) {
					strctFields := getFieldParams(v.Name.Name, strct)
					generateUnmarshaller(out, strctFields)

				}
			}
		}
		return true
	})

	for k, v := range servers {
		generateServeHTTP(out, k, v)
	}
}

type FieldParams struct {
	Required bool
	Type     string
	JsonName string
	Name     string
	Min      string
	Max      string
	Enum     string
	Default  string
}
type StrctFields struct {
	Name   string
	Params []*FieldParams
}

func generateUnmarshaller(w io.Writer, s *StrctFields) {
	unmarshallTmpl := template.Must(template.New("unmarshallTmpl").Parse(`
func (in *{{.Name}}) Unmarshall (q url.Values) error {
	{{template "validateTmpl" .}}
	return nil
}
			`,
	))
	validateTmpl := template.Must(unmarshallTmpl.Parse(`
	{{define "validateTmpl"}}
	{{range .Params}}
	{{template "getTmpl" .}}
	{{template "requiredTmpl" .}}
	{{template "defaultTmpl" .}}
	{{template "enumTmpl" .}}
	{{template "minTmpl" .}}
	{{template "maxTmpl" .}}
	{{end}}
	{{end}}
		`,
	))

	template.Must(validateTmpl.Parse(`
	{{define "getTmpl"}}
	{{if (eq .Type "int")}}
	p{{.Name}}, err := strconv.Atoi(q.Get("{{.JsonName}}"))
	if err != nil {
		return fmt.Errorf("{{.JsonName}} must be int")
	}
	in.{{.Name}} = p{{.Name}}
	{{else}}
	in.{{.Name}} = q.Get("{{.JsonName}}")
	{{end}}
	{{end}}
	`,
	))

	template.Must(validateTmpl.Parse(`
	{{define "requiredTmpl"}}
	{{if .Required}}
	{{if (eq .Type "int")}}
if in.{{.Name}} == 0 {
	return fmt.Errorf("{{.JsonName}} must me not empty")
}
	{{else}}
if in.{{.Name}} == "" {
	return fmt.Errorf("{{.JsonName}} must me not empty")
}
	{{end -}}
	{{end -}}
	{{end -}}
	`,
	))

	template.Must(validateTmpl.Parse(`
	{{define "defaultTmpl"}}
	{{if not (eq .Default "")}}
	if in.{{.Name}} == ""{
		in.{{.Name}}="{{.Default}}"
	}
	{{end -}}
	{{end -}}
	`,
	))

	template.Must(validateTmpl.Parse(`
	{{define "enumTmpl"}}
	{{if not (eq .Enum "")}}
	if !strings.Contains("|{{.Enum}}|", "|"+in.{{.Name}}+"|"){
		enum := "[" + strings.Replace("{{.Enum}}", "|", ", ", -1) + "]"
		return fmt.Errorf("{{.JsonName}} must be one of "+enum)
	}
	{{end -}}
	{{end -}}
	`,
	))
	template.Must(validateTmpl.Parse(`
	{{define "minTmpl"}}
	{{if not (eq .Min "")}}
	{{if (eq .Type "int")}}
	if in.{{.Name}} < {{.Min}}{
		return fmt.Errorf("{{.JsonName}} must be >= {{.Min}}") 
	}
	{{else}}
	if len(in.{{.Name}}) < {{.Min}}{
		return fmt.Errorf("{{.JsonName}} len must be >= {{.Min}}") 
	}
	{{end -}}
	{{end -}}
	{{end -}}
	`,
	))

	template.Must(validateTmpl.Parse(`
	{{define "maxTmpl"}}
	{{if not (eq .Max "")}}
	{{if (eq .Type "int")}}
	if in.{{.Name}} > {{.Max}}{
		return fmt.Errorf("{{.JsonName}} must be <= {{.Max}}") 
	}
	{{else}}
	if len(in.{{.Name}}) > {{.Max}}{
		return fmt.Errorf("{{.JsonName}} len must be <= {{.Max}}") 
	}
	{{end -}}
	{{end -}}
	{{end -}}
	`,
	))

	err := unmarshallTmpl.Execute(w, s)
	if err != nil {
		log.Fatal(err)
	}
}

func getFieldParams(sName string, s *ast.StructType) *StrctFields {
	strctFields := &StrctFields{}

	strctFields.Name = sName
	for _, field := range s.Fields.List {
		fParams := &FieldParams{}
		fParams.Type = field.Type.(*ast.Ident).Name
		fParams.Name = field.Names[0].Name

		//Starting parsing tag
		params := parseTags(field.Tag.Value)
		fParams.Required = getRequired(params)
		fParams.JsonName = getParamName(fParams.Name, params)
		fParams.Default = getDefaultName(params)
		fParams.Enum = getEnum(params)
		fParams.Min = getMin(params)
		fParams.Max = getMax(params)
		strctFields.Params = append(strctFields.Params, fParams)
	}
	return strctFields
}

func getMax(params []string) string {
	target := "max="
	for _, v := range params {
		if strings.Contains(v, target) {
			return strings.Replace(v, target, "", 1)
		}
	}
	return ""
}
func getMin(params []string) string {
	target := "min="
	for _, v := range params {
		if strings.Contains(v, target) {
			return strings.Replace(v, target, "", 1)
		}
	}
	return ""
}
func getEnum(params []string) string {
	target := "enum="
	for _, v := range params {
		if strings.Contains(v, target) {
			return strings.Replace(v, target, "", 1)
		}
	}
	return ""
}
func getParamName(name string, params []string) string {
	target := "paramname="
	for _, v := range params {
		if strings.Contains(v, target) {
			return strings.Replace(v, target, "", 1)
		}
	}
	return ToSnakeCase(name)
}
func getDefaultName(params []string) string {
	target := "default="
	for _, v := range params {
		if strings.Contains(v, target) {
			return strings.Replace(v, target, "", 1)
		}
	}
	return ""
}

func ToSnakeCase(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func getRequired(params []string) bool {
	for _, v := range params {
		if strings.Contains(v, "required") {
			return true
		}
	}
	return false
}

func parseTags(tag string) []string {
	replacer := strings.NewReplacer("\"", "", "`", "")
	return strings.Split(
		replacer.Replace(
			strings.TrimSpace(
				strings.Replace(
					tag, "apivalidator:", "", 1,
				),
			),
		),
		",",
	)
}

func isParamStruct(strct *ast.StructType) bool {
	if len(strct.Fields.List) < 1 {
		return false
	}

	if strct.Fields.List[0].Tag == nil {
		return false
	}

	if !strings.Contains(strct.Fields.List[0].Tag.Value, "apivalidator:") {
		return false
	}
	return true
}

func generateServeHTTP(w io.Writer, srvName string, srvInfos []*ServerInfo) {
	p := struct {
		Rcv   string
		Infos []*ServerInfo
	}{
		Rcv:   srvName,
		Infos: srvInfos,
	}

	serveHttpTmpl.Execute(w, p)
}

type Handler struct {
	// Parameters *Params
	Name        string
	RecName     string
	Method      string
	InputParams string
	Auth        bool
}

func generateHandler(w io.Writer, f *ast.FuncDecl, auth bool, method, srvName string) *Handler {
	h := &Handler{}
	h.Method = method
	h.Name = f.Name.Name
	h.RecName = srvName
	h.Auth = auth

	for _, p := range f.Type.Params.List {
		switch t := p.Type.(type) {
		case *ast.Ident:
			h.InputParams = t.Name
		}
	}
	fmt.Println(h)
	handlerTmpl.Execute(w, h)
	fmt.Println("***************")
	return h
}

func getReceiverName(f *ast.FuncDecl) string {
	rec, ok := f.Recv.List[0].Type.(*ast.StarExpr).X.(*ast.Ident)
	if !ok {
		return ""
	}
	return rec.Name
}

type Params struct {
	Url    string
	Auth   bool
	Method string
}

func generateHttpParamsForServer(doc string) *Params {
	paramsStr := strings.TrimSpace(strings.Replace(doc, "apigen:api", "", 1))
	p := &Params{}
	json.Unmarshal([]byte(paramsStr), p)
	return p
}
